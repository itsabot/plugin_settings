package main

import (
	"flag"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/log"
	"github.com/itsabot/abot/shared/nlp"
	"github.com/itsabot/abot/shared/pkg"
	"github.com/itsabot/abot/shared/task"
	"github.com/jmoiron/sqlx"
)

type Settings string

var vocab dt.Vocab
var db *sqlx.DB
var p *pkg.Pkg
var l *log.Logger

const pkgName string = "settings"
const (
	stateInvalid int = iota
	stateAddCard
	stateChangeCard
	stateChangeCalendar
	stateAddAddress
)

func main() {
	var coreaddr string
	flag.StringVar(&coreaddr, "coreaddr", "",
		"Port used to communicate with Abot.")
	flag.Parse()
	l = log.New(pkgName)
	rand.Seed(time.Now().UnixNano())
	var err error
	db, err = pkg.ConnectDB()
	if err != nil {
		l.Fatal(err)
	}
	trigger := &nlp.StructuredInput{
		Commands: []string{"change", "modify", "switch", "alter", "add",
			"remove", "delete"},
		Objects: []string{"card", "address", "calendar"},
	}
	p, err = pkg.NewPackage(pkgName, coreaddr, trigger)
	if err != nil {
		l.Fatal("building", err)
	}
	p.Vocab = dt.NewVocab(
		// TODO change handlers to use triggers
		dt.VocabHandler{
			Fn:       kwAddCard,
			WordType: "Object",
			Words:    []string{"card"},
		},
		dt.VocabHandler{
			Fn:       kwChangeCard,
			WordType: "Command",
			Words: []string{"change", "modify", "delete", "switch",
				"alter"},
		},
		dt.VocabHandler{
			Fn:       kwChangeCalendar,
			WordType: "Object",
			Words:    []string{"calendar", "cal", "schedule", "rota"},
		},
		dt.VocabHandler{
			Fn:       kwAddAddress,
			WordType: "Object",
			Words:    []string{"address", "addr"},
		},
	)
	settings := new(Settings)
	if err := p.Register(settings); err != nil {
		l.Fatal("registering", err)
	}
}

func (t *Settings) Run(in *dt.Msg, resp *string) error {
	sm := bootStateMachine(in)
	sm.SetOnReset(func(in *dt.Msg) {
		sm.SetMemory(in, "state", stateInvalid)
	})
	sm.SetMemory(in, "__state_entered", false)
	return handleInput(in, resp)
}

func (t *Settings) FollowUp(in *dt.Msg, resp *string) error {
	return handleInput(in, resp)
}

func handleInput(in *dt.Msg, resp *string) error {
	sm := bootStateMachine(in)
	sm.SetOnReset(func(in *dt.Msg) {
		sm.SetMemory(in, "state", stateInvalid)
	})
	*resp = p.Vocab.HandleKeywords(in)
	if len(*resp) == 0 {
		state := int(sm.GetMemory(in, "state").Int64())
		switch state {
		case stateAddCard:
			l.Debug("setting state addCard")
			sm.SetStates(addCard)
		case stateChangeCard:
			l.Debug("setting state changeCard")
			sm.SetStates(changeCard)
		case stateChangeCalendar:
			l.Debug("setting state changeCalendar")
			sm.SetStates(changeCalendar)
		case stateAddAddress:
			l.Debug("setting state changeCalendar")
			sm.SetStates(addShippingAddress(in))
		default:
			l.Debug("unrecognized state", state)
		}
		*resp = sm.Next(in)
	}
	return nil
}

func kwAddCard(in *dt.Msg, _ int) string {
	sm := bootStateMachine(in)
	sm.SetMemory(in, "state", stateAddCard)
	return ""
}

func kwChangeCard(in *dt.Msg, _ int) string {
	sm := bootStateMachine(in)
	sm.SetMemory(in, "state", stateChangeCard)
	return ""
}

func kwChangeCalendar(in *dt.Msg, _ int) string {
	sm := bootStateMachine(in)
	sm.SetMemory(in, "state", stateChangeCalendar)
	return ""
}

func kwAddAddress(in *dt.Msg, _ int) string {
	sm := bootStateMachine(in)
	sm.SetMemory(in, "state", stateAddAddress)
	return ""
}

func bootStateMachine(in *dt.Msg) *dt.StateMachine {
	sm := dt.NewStateMachine(pkgName)
	sm.SetDBConn(db)
	sm.SetLogger(l)
	sm.LoadState(in)
	return sm
}

var addCard []dt.State = []dt.State{
	{
		OnEntry: func(in *dt.Msg) string {
			url := path.Join(os.Getenv("ABOT_URL"), "cards", "new")
			return "You can add your card securely here: " + url
		}, OnInput: func(in *dt.Msg) {
		},
		Complete: func(in *dt.Msg) (bool, string) {
			return true, ""
		},
	},
}

var changeCalendar []dt.State = []dt.State{
	{
		OnEntry: func(in *dt.Msg) string {
			url := path.Join(os.Getenv("ABOT_URL"), "profile")
			return "You can connect your Google calendar on your profile: " + url
		},
		OnInput: func(in *dt.Msg) {
		},
		Complete: func(in *dt.Msg) (bool, string) {
			return true, ""
		},
	},
}

var changeCard []dt.State = []dt.State{
	{
		OnEntry: func(in *dt.Msg) string {
			url := path.Join(os.Getenv("ABOT_URL"), "profile")
			return "You can change your cards securely here: " + url
		}, OnInput: func(in *dt.Msg) {
		},
		Complete: func(in *dt.Msg) (bool, string) {
			return true, ""
		},
	},
}

func addShippingAddress(in *dt.Msg) []dt.State {
	sm := bootStateMachine(in)
	sm.SetMemory(in, "state", stateAddAddress)
	return task.New(sm, task.RequestAddress, "shipping_address")
}
