package main

import (
	"context"
	"fmt"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/events"
	"github.com/SevereCloud/vksdk/v2/longpoll-bot"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"time"
)

import _ "github.com/joho/godotenv/autoload"

var TOKEN = os.Getenv("TOKEN")
var CHAT_NUMBER = os.Getenv("CHAT_NUMBER")
var CHAT_ID, _ = strconv.Atoi(os.Getenv("CHAT_ID"))
var CONTROL_CHAT_NUMBER = os.Getenv("CONTROL_CHAT_NUMBER")
var CONTROL_CHAT_ID, _ = strconv.Atoi(os.Getenv("CONTROL_CHAT_ID"))

type CommandParams map[string]string

type CommandDirector struct {
	helper        *Helper
	groupCommands map[int][]CommandHandler
}

func (d *CommandDirector) Direct(text string, group int, object events.MessageNewObject) {
	v, ok := d.groupCommands[group]
	if ok {
		for _, commandHandler := range v {
			if commandHandler.canProcess(text) {
				commandHandler.process(text, d.helper, object)
			}
		}
	}
}

type Helper struct {
	vk *api.VK
	db *gorm.DB
}

func (a *Helper) SendMessage(group int, message string) {
	SendMessage(a.vk, group, message)
}

func SendMessage(vk *api.VK, group int, message string) {
	vk.MessagesSend(api.Params{
		"peer_id":   group,
		"message":   message,
		"random_id": rand.Int(),
	})
}

type CommandHandler struct {
	rx      *Rx
	handler func(h *Helper, cp CommandParams, object events.MessageNewObject)
}

func (ch *CommandHandler) canProcess(text string) bool {
	return ch.rx.match(text)
}

func (ch *CommandHandler) process(text string, helper *Helper, object events.MessageNewObject) {
	if ch.rx.Retrieve != nil {
		ch.handler(helper, ch.rx.getTextParams(text), object)
	} else {
		ch.handler(helper, CommandParams{}, object)
	}
}

type Rx struct {
	Match    *regexp.Regexp
	Retrieve *regexp.Regexp
}

func (rx *Rx) match(text string) bool {
	return rx.Match.MatchString(text)
}

func (rx *Rx) getTextParams(text string) CommandParams {
	names := rx.Retrieve.SubexpNames()
	matches := rx.Retrieve.FindStringSubmatch(text)
	m := CommandParams{}
	for i := 1; i < len(matches); i++ {
		m[names[i]] = matches[i]
	}
	return m
}

func main() {

	db, err := gorm.Open(sqlite.Open(os.Getenv("DB")))
	if err != nil {
		fmt.Println("Db Errr")
		fmt.Println(err)
		panic("no connection to db")
	}

	var groupCommands = map[int][]CommandHandler{
		CHAT_ID: {
			CommandHandler{
				rx: &Rx{
					Match:    regexp.MustCompile(`^цитаты по автору\s\S+$`),
					Retrieve: regexp.MustCompile(`^цитаты по автору\s(?<author>\S+)$`),
				},
				handler: func(h *Helper, cp CommandParams, object events.MessageNewObject) {
					quote := fmt.Sprintf("%%%v%%", cp["author"])
					rows, _ := h.db.Raw(
						"select id, text, author from quote where author like ? order by id desc",
						quote).Rows()
					defer rows.Close()
					var id int
					var text, author string
					var result = ""
					for rows.Next() {
						rows.Scan(&id, &text, &author)
						result += fmt.Sprintf("%v. %v © %v\n", id, text, author)
					}
					message := ""
					if len(result) > 0 {
						message = result
					} else {
						message = "Нет цитат по такому автору"
					}
					h.SendMessage(CHAT_ID, message)
				},
			},
			CommandHandler{
				rx: &Rx{
					Match: regexp.MustCompile(`^летопись$`),
				},
				handler: func(h *Helper, cp CommandParams, object events.MessageNewObject) {
					rows, _ := h.db.Raw("select strftime('%d.%m.%Y', dt), event from story order by dt desc").Rows()
					defer rows.Close()
					var dt, event, result, chunk string
					for rows.Next() {
						rows.Scan(&dt, &event)
						chunk = fmt.Sprintf("%v - %v\n", dt, event)
						if len(chunk)+len(result) > 9000 {
							h.SendMessage(CHAT_ID, result)
							result = ""
						} else {
							result += chunk
						}
					}
					message := ""
					if len(result) > 0 {
						message = result
					} else {
						message = "Нет событий"
					}
					h.SendMessage(CHAT_ID, message)
				},
			},
			CommandHandler{
				rx: &Rx{
					Match:    regexp.MustCompile(`^\+цитата\s+(.+)$`),
					Retrieve: regexp.MustCompile(`^\+цитата\s+(?<author>.+)$`),
				},
				handler: func(h *Helper, cp CommandParams, object events.MessageNewObject) {
					h.db.Exec(
						"insert into quote (text, author, original_author) values (?, ?, ?)",
						object.Message.ReplyMessage.Text, cp["author"], object.Message.FromID)
				},
			},
			CommandHandler{
				rx: &Rx{
					Match:    regexp.MustCompile(`^\+летопись\s+(.+)$`),
					Retrieve: regexp.MustCompile(`^\+летопись\s+(?<event>(.+))$`),
				},
				handler: func(h *Helper, cp CommandParams, object events.MessageNewObject) {
					h.db.Exec(
						"insert into story (event, dt) values(?, datetime())", cp["event"])
					h.SendMessage(CHAT_ID, "Добавили событие")
				},
			},
		},
	}

	var vk = api.NewVK(TOKEN)
	lp, err := longpoll.NewLongPoll(vk, 205920300)

	if err != nil {
		panic("Long pool error")
	}

	commandDirector := CommandDirector{
		helper:        &Helper{vk: vk, db: db},
		groupCommands: groupCommands,
	}

	lp.MessageNew(func(ctx context.Context, object events.MessageNewObject) {
		messageText := object.Message.Text
		from := object.Message.PeerID
		commandDirector.Direct(messageText, from, object)
		if rand.Intn(100) < 20 {
			var response int
			vk.RequestUnmarshal("messages.sendReaction", response, api.Params{
				"peer_id":     object.Message.PeerID,
				"cmid":        object.Message.ConversationMessageID,
				"reaction_id": rand.Intn(16),
			})
		}
	})

	fmt.Println(rand.Intn(10))

	quoteTick := time.Tick(6 * time.Hour)
	activityTick := time.Tick(30 * time.Second)

	go func() {
		for {
			select {
			case <-quoteTick:
				row := db.Raw("select text, author from quote order by random() limit 1").Row()
				var text, author string
				row.Scan(&text, &author)
				SendMessage(vk, CHAT_ID, fmt.Sprintf("%v © %v", text, author))
				break
			default:
				time.Sleep(30 * time.Minute)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-activityTick:
				var activity string
				if rand.Intn(10) > 5 {
					activity = "audiomessage"
				} else {
					activity = "typing"
				}
				vk.MessagesSetActivity(api.Params{
					"peer_id": CHAT_ID,
					"type":    activity,
				})
			default:
				time.Sleep(10 * time.Second)
			}
		}
	}()

	lp.Run()
}
