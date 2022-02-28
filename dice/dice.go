package dice

import (
	"errors"
	wr "github.com/mroth/weightedrand"
	"math/rand"
	"os"
	"sealdice-core/core"
	"time"
)

var VERSION = "0.91测试版 v20220227"

type CmdExecuteResult struct {
	Success bool
}

type CmdItemInfo struct {
	Name  string
	Solve func(ctx *MsgContext, msg *Message, cmdArgs *CmdArgs) CmdExecuteResult
	Brief string
	Help  string
}

type CmdMapCls map[string]*CmdItemInfo

type ExtInfo struct {
	Name    string // 名字
	Version string // 版本
	// 作者
	// 更新时间
	AutoActive      bool      // 是否自动开启
	CmdMap          CmdMapCls `yaml:"-"` // 指令集合
	Brief           string    `yaml:"-"`
	ActiveOnPrivate bool      `yaml:"-"`

	Author string `yaml:"-"`
	//activeInSession bool; // 在当前会话中开启

	OnCommandReceived func(ctx *MsgContext, msg *Message, cmdArgs *CmdArgs)                             `yaml:"-"`
	OnMessageReceived func(ctx *MsgContext, msg *Message)                                               `yaml:"-"`
	OnMessageSend     func(ctx *MsgContext, messageType string, userId int64, text string, flag string) `yaml:"-"`
	GetDescText       func(i *ExtInfo) string                                                           `yaml:"-"`
	IsLoaded          bool                                                                              `yaml:"-"`
	OnLoad            func()                                                                            `yaml:"-"`
}

type Dice struct {
	ImSession             *IMSession             `yaml:"imSession"`
	CmdMap                CmdMapCls              `yaml:"-"`
	ExtList               []*ExtInfo             `yaml:"-"`
	RollParser            *DiceRollParser        `yaml:"-"`
	CommandCompatibleMode bool                   `yaml:"commandCompatibleMode"`
	LastSavedTime         *time.Time             `yaml:"lastSavedTime"`
	TextMap               map[string]*wr.Chooser `yaml:"-"`
	ConfigVersion         int                    `yaml:"configVersion"`
}

func (d *Dice) Init() {
	os.MkdirAll("./data/configs", 0644)
	os.MkdirAll("./data/extensions", 0644)
	os.MkdirAll("./data/logs", 0644)

	d.CommandCompatibleMode = true
	d.ImSession = &IMSession{}
	d.ImSession.Parent = d
	d.ImSession.ServiceAt = make(map[int64]*ServiceAtItem)
	d.CmdMap = CmdMapCls{}

	d.registerCoreCommands()
	d.RegisterBuiltinExt()
	d.loads()

	for _, i := range d.ExtList {
		if i.OnLoad != nil {
			i.OnLoad()
		}
	}

	autoSave := func() {
		t := time.Tick(30 * time.Second)
		for {
			<-t
			d.save()
		}
	}
	go autoSave()

	refreshGroupInfo := func() {
		t := time.Tick(35 * time.Second)
		defer func() {
			// 防止报错
			if r := recover(); r != nil {
				core.GetLogger().Error(r)
			}
		}()

		for {
			<-t
			if d.ImSession.Socket != nil {
				for k := range d.ImSession.ServiceAt {
					GetGroupInfo(d.ImSession.Socket, k)
				}
			}
		}
	}
	go refreshGroupInfo()
}

func (d *Dice) rebuildParser(buffer string) *DiceRollParser {
	p := &DiceRollParser{Buffer: buffer}
	_ = p.Init()
	p.RollExpression.Init(255)
	//d.RollParser = p;
	return p
}

func (d *Dice) ExprEvalBase(buffer string, ctx *MsgContext, bigFailDice bool) (*VmResult, string, error) {
	parser := d.rebuildParser(buffer)
	err := parser.Parse()
	parser.RollExpression.BigFailDiceOn = bigFailDice

	if err == nil {
		parser.Execute()
		if parser.Error != nil {
			return nil, "", parser.Error
		}
		num, detail, _ := parser.Evaluate(d, ctx)
		ret := VmResult{}
		ret.Value = num.Value
		ret.TypeId = num.TypeId
		ret.Parser = parser
		return &ret, detail, nil
	}
	return nil, "", err
}

func (d *Dice) ExprEval(buffer string, ctx *MsgContext) (*VmResult, string, error) {
	return d.ExprEvalBase(buffer, ctx, false)
}

func (d *Dice) ExprText(buffer string, ctx *MsgContext) (string, string, error) {
	val, detail, err := d.ExprEval("`"+buffer+"`", ctx)

	if err == nil && val.TypeId == VMTypeString {
		return val.Value.(string), detail, err
	}

	return "", "", errors.New("错误的表达式")
}

func DiceRoll(dicePoints int) int {
	if dicePoints <= 0 {
		return 0
	}
	val := rand.Int()%dicePoints + 1
	return val
}

func DiceRoll64(dicePoints int64) int64 {
	if dicePoints == 0 {
		return 0
	}
	val := rand.Int63()%dicePoints + 1
	return val
}
