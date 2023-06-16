package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/irc.v3"
)

var (
	logger         *logrus.Logger
	newMessage     chan *irc.Message
	messageCounter uint32
	cfg            Config
	life           state
)

const (
	overwriteSubCountCommand = "!nonprimesubcount "
)

type Config struct {
	DebugLog       bool   `json:"debug_log,omitempty"`
	DebugInputFile string `json:"debug_input_file,omitempty"`

	IRCAddress string `json:"irc_address"`
	IRCUser    string `json:"irc_user"`
	IRCChannel string `json:"irc_channel"`

	// if this file exists, we read the initial non-prime subcount from it.
	// then, we keep it updated with the latest non-prime subcount.
	// use "!nonprimesubcount 0" in chat to reset it.
	OutputFile string `json:"output_file"`
}

func main() {
	newMessage = make(chan *irc.Message, 16)
	logger = logrus.New()

	cfgPath := os.Getenv("EZNOPRIMES_CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.json"
	}
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(&Config{})
		logger.WithError(err).WithField("cfg-path", cfgPath).Fatal("failed to load config, please fill above sample and save to cfg-path")
	}

	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		logger.WithError(err).Fatal("failed to load cfg, please check syntax")
	}

	if cfg.DebugLog {
		logger.SetLevel(logrus.DebugLevel)
	}

	logger.WithField("address", cfg.IRCAddress).Info("connecting")
	conn, err := reliableDial("tcp", cfg.IRCAddress, 5)
	if err != nil {
		logger.WithError(err).Fatal("failed to dial IRC address multiple times, aborting")
	}

	config := irc.ClientConfig{
		Nick:    cfg.IRCUser,
		User:    cfg.IRCUser,
		Name:    cfg.IRCUser,
		Handler: irc.HandlerFunc(ircHandler),
	}

	loadState()
	go handleMessages()
	go printAfterFirstMessage()
	go printMessageVelocityEvery15Mins()

	client := irc.NewClient(conn, config)
	go importDebugInputFile(client)
	if err := client.Run(); err != nil {
		logger.WithError(err).Fatal("failure during IRC run")
	}
}

func reliableDial(network, address string, maxAttempts int) (conn net.Conn, err error) {
	attempts := 0
	for {
		conn, err = net.Dial(network, address)
		if err == nil {
			return
		}
		attempts++
		if attempts >= maxAttempts {
			return
		}
		time.Sleep(time.Second)
	}
}

func ircHandler(c *irc.Client, m *irc.Message) {
	logger.WithField("message", m.String()).Debug("received message")
	if m.Command == "001" {
		// 001 is a welcome event, so start our setup process
		if err := c.Write("CAP REQ :twitch.tv/tags twitch.tv/commands"); err != nil {
			logger.WithError(err).Fatal("failed to request caps")
		}
		if err := c.Write("JOIN #" + cfg.IRCChannel); err != nil {
			logger.WithError(err).Fatal("failed to join channel")
		}
		logger.WithField("channel", cfg.IRCChannel).Info("knock knock")
	} else if m.Command == "ROOMSTATE" && m.Trailing() == "#"+cfg.IRCChannel {
		// emitted when a room join is successful
		logger.WithField("channel", cfg.IRCChannel).Info("party time")
	} else if (m.Command == "PRIVMSG" || m.Command == "USERNOTICE") && c.FromChannel(m) {
		// regular chat message or event
		newMessage <- m
		atomic.AddUint32(&messageCounter, 1)
	}
}

func importDebugInputFile(c *irc.Client) {
	if cfg.DebugInputFile == "" {
		return
	}

	handle, err := os.Open(cfg.DebugInputFile)
	if err != nil {
		logger.WithError(err).Warn("failed to open debug input file, ignoring replay")
		return
	}
	defer handle.Close()

	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		line := scanner.Text()
		message, err := irc.ParseMessage(line)
		if err != nil {
			logger.WithError(err).WithField("line", line).Warn("invalid message, ignoring replay")
			return
		}

		ircHandler(c, message)
		logger.WithField("line", line).Info("replayed debug input")
	}

	logger.Info("finished replaying debug input")
}

// print something after we receive our first message, so we know stuff kinda works
func printAfterFirstMessage() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		if atomic.AddUint32(&messageCounter, 0) > 0 {
			logger.WithField("channel", cfg.IRCChannel).Info("received first regular chat message")
			return
		}
	}
}

// print message velocity every x mins, unless 0, so we know stuff continues to work
func printMessageVelocityEvery15Mins() {
	ticker := time.NewTicker(time.Minute * 15)
	defer ticker.Stop()

	for {
		<-ticker.C
		cCtr := atomic.SwapUint32(&messageCounter, 0)
		if cCtr == 0 {
			continue
		}
		logger.WithField("messages", cCtr).Info("past 15 min")
	}
}

// messages can add or overwrite the sub count
type outcome struct {
	incrementSubs bool
	overwriteSubs bool
	subs          int
}

func messageOutcome(m *irc.Message) outcome {
	var outcome outcome

	if m.Command == "USERNOTICE" {
		// this should be a non-prime sub or resub message
		msgID, _ := m.Tags.GetTag("msg-id")
		if msgID == "sub" || msgID == "resub" {
			msgSubPlan, _ := m.Tags.GetTag("msg-param-sub-plan")
			if msgSubPlan != "Prime" {
				outcome.incrementSubs = true
				outcome.subs = 1
			}
		}
	}

	if m.Command == "PRIVMSG" {
		trailing := m.Trailing()
		if len(trailing) > 0 && trailing[0] == '!' {
			// maybe command, must be mod or broadcaster
			mod, _ := m.Tags.GetTag("mod")
			badges, _ := m.Tags.GetTag("badges")
			broadcaster := strings.Contains(badges, "broadcaster/1") // idk how to check otherwise
			if mod == "1" || broadcaster {
				if strings.HasPrefix(trailing, overwriteSubCountCommand) {
					amt, err := strconv.Atoi(trailing[len(overwriteSubCountCommand):])
					if err != nil {
						logger.WithError(err).WithField("trailing", trailing).Warn("failed to parse amounts from overwrite command")
					} else {
						outcome.overwriteSubs = true
						outcome.subs = amt
					}
				}
			}
		}
	}

	return outcome
}

// our program tracks the subcount
type state struct {
	subs int
}

// when it is updated, write the new value to disk
type action struct {
	writeSubs bool
}

func (s *state) MergeOutcome(o outcome) action {
	var action action

	if o.incrementSubs {
		s.subs += o.subs
		action.writeSubs = true
	}

	if o.overwriteSubs {
		s.subs = o.subs
		action.writeSubs = true
	}

	return action
}

func performAction(action action) {
	if action.writeSubs {
		err := os.WriteFile(cfg.OutputFile, []byte(strconv.Itoa(life.subs)), os.ModePerm)
		if err != nil {
			logger.WithError(err).Warn("failed to write subcount to output file")
		} else {
			logger.WithField("subs", life.subs).Info("wrote subcount")
		}
	}
}

func handleMessage(m *irc.Message) {
	outcome := messageOutcome(m)
	action := life.MergeOutcome(outcome)
	performAction(action)
}

func handleMessages() {
	for msg := range newMessage {
		handleMessage(msg)
	}
}

func loadState() {
	contents, err := os.ReadFile(cfg.OutputFile)
	if os.IsNotExist(err) {
		os.WriteFile(cfg.OutputFile, []byte("0"), os.ModePerm)
		logger.Info("previous subcount not found, starting at 0, saved new file")
		return
	} else if err != nil {
		logger.WithError(err).WithField("output-file", cfg.OutputFile).Warn("failed to read file, ignoring, starting at 0")
		return
	}

	subs, err := strconv.Atoi(string(contents))
	if err != nil {
		logger.WithError(err).WithField("contents", contents).Warn("failed to parse previous subcount, ignoring, starting at 0")
		return
	}

	life.MergeOutcome(outcome{
		overwriteSubs: true,
		subs:          subs,
	})
	logger.WithField("subcount", subs).Info("loaded previous subcount")
}
