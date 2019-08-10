package janet

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/troyxmccall/janet/database"
	"github.com/troyxmccall/janet/munge"
	"github.com/troyxmccall/janet/ui"

	"github.com/aybabtme/log"
	"github.com/dustin/go-humanize"
	"github.com/nlopes/slack"
)

var (
	regexps = struct {
		Motivate, GivePoints, TakePoints, QueryPoints, Leaderboard, URL, SlackUser, Throwback *regexp.Regexp
	}{
		Motivate:    karmaReg.MatchMotivate(),
		GivePoints:  karmaReg.MatchGive(),
		TakePoints:  karmaReg.MatchTake(),
		QueryPoints: karmaReg.MatchQuery(),
		Leaderboard: regexp.MustCompile(`^goodplace(?)? (?:leaderboard|top|highscores) ?([0-9]+)?$`),
		URL:         regexp.MustCompile(`^janet(?:bot)? (?:url|web|link)?$`),
		SlackUser:   regexp.MustCompile(`^<@([A-Za-z0-9]+)>$`),
		Throwback:   karmaReg.MatchThrowback(),
	}
)

// Database is an abstraction around the database, mostly designed for use in tests.
type Database interface {
	// InsertPoints persistently records that points have been given or deducted.
	InsertPoints(points *database.Points) error

	// GetUser returns information about a user, including their current number of points.
	GetUser(name string) (*database.User, error)

	// GetLeaderboard returns the top X users with the most points, in order.
	GetLeaderboard(limit int) (database.Leaderboard, error)

	// GetTotalPoints returns the total number of points transferred across all users.
	GetTotalPoints() (int, error)

	// GetThrowback returns a random karma operation on a specific user.
	GetThrowback(user string) (*database.Throwback, error)
}

// ChatService is an abstraction around Slack, mostly designed for use in tests.
type ChatService interface {
	// IncomingEventsChan returns a channel of real-time events.
	IncomingEventsChan() chan slack.RTMEvent

	// NewOutgoingMessage constructs a new OutgoingMessage using the provided text and channel.
	NewOutgoingMessage(text string, channel string, options ...slack.RTMsgOption) *slack.OutgoingMessage

	// SendMessage sends the provided *OutgoingMessage.
	SendMessage(msg *slack.OutgoingMessage)

	// OpenIMChannel opens a new direct-message channel with the specified user.
	// It returns some status information, and the channel ID.
	OpenIMChannel(user string) (bool, bool, string, error)

	// GetUserInfo retrieves the complete user information for the specified username.
	GetUserInfo(user string) (*slack.User, error)
}

// SlackChatService is an implementation of ChatService using github.com/nlopes/slack.
type SlackChatService struct {
	slack.RTM
}

// IncomingEventsChan returns a channel of real-time messaging events.
func (s SlackChatService) IncomingEventsChan() chan slack.RTMEvent {
	return s.IncomingEvents
}

// UserAliases is a map of alias -> main username
type UserAliases map[string]string

// ReactjiConfig contains the configuration for reactji-based votes
type ReactjiConfig struct {
	Enabled          bool
	Upvote, Downvote StringList
}

// Config contains all the necessary configs for janet.
type Config struct {
	Slack                       ChatService
	BadJanetSlack               ChatService
	Debug, Motivate, SelfPoints bool
	MaxPoints, LeaderboardLimit int
	Log                         *log.Log
	UI                          ui.Provider
	DB                          Database
	UserBlacklist               StringList
	Aliases                     UserAliases
	Reactji                     *ReactjiConfig
	WaitGroup                   *sync.WaitGroup
}

// A Bot is an instance of janet.
type Bot struct {
	Config    *Config
	WaitGroup *sync.WaitGroup
}

// New returns a pointer to an new instance of janet.
func New(config *Config) *Bot {
	return &Bot{
		Config: config,
	}
}

func (b *Bot) Listen() {

	//b.Config.Log.Info("listener called")

	//add listenrs to waitgroup because they need to run at the same timee (concurrently)
	var wg sync.WaitGroup
	wg.Add(2)

	b.GoodJanetListen(wg)
	b.BadJanetListen(wg)

	wg.Wait()
}

// Listen starts listening for Slack messages and calls the
// appropriate handlers.
func (b *Bot) GoodJanetListen(wg sync.WaitGroup) {

	//b.Config.Log.Info("good-janet listener called")

	go func() {
		for msg := range b.Config.Slack.IncomingEventsChan() {
			switch ev := msg.Data.(type) {
			case *slack.ReactionAddedEvent:
				go b.handleReactionAddedEvent(msg.Data.(*slack.ReactionAddedEvent))
			case *slack.ReactionRemovedEvent:
				go b.handleReactionRemovedEvent(msg.Data.(*slack.ReactionRemovedEvent))
			case *slack.MessageEvent:
				go b.handleMessageEvent(msg.Data.(*slack.MessageEvent))
			case *slack.ConnectedEvent:
				b.Config.Log.Info("janet connected to slack")
				if b.Config.Debug {
					b.Config.Log.KV("info", ev.Info).Info("got slack info")
					b.Config.Log.KV("connections", ev.ConnectionCount).Info("got connection count")
				}
			case *slack.RTMError:
      b.Config.Log.Err(ev).Error("slack rtm error")
    case *slack.InvalidAuthEvent:
      b.Config.Log.Fatal("invalid slack token")
    default:
      b.Config.Log.KV("data", msg.Data).KV("event", reflect.TypeOf(msg.Data)).Info("unexpected slack api event")
			}
		}
	}()
}

func (b *Bot) BadJanetListen(wg sync.WaitGroup) {

	//b.Config.Log.Info("bad-janet listener called")

	go func() {
		for msg := range b.Config.BadJanetSlack.IncomingEventsChan() {
			switch ev := msg.Data.(type) {
			case *slack.MessageEvent:
				b.Config.Log.Info("bad-janet got a message")
				//go b.handleMessageEvent(msg.Data.(*slack.MessageEvent))
			case *slack.ConnectedEvent:
				b.Config.Log.Info("bad-janet connected to slack")
				if b.Config.Debug {
					b.Config.Log.KV("info", ev.Info).Info("got bad-janet slack info")
					b.Config.Log.KV("connections", ev.ConnectionCount).Info("got bad-janet connection count")
				}
			case *slack.RTMError:
				b.Config.Log.Err(ev).Error("bad-janet slack rtm error")
			case *slack.InvalidAuthEvent:
				wg.Done()
				b.Config.Log.Fatal("bad-janet invalid slack token")
			default:
			}
		}
	}()
}

// SendMessage sends a message to a Slack channel.
func (b *Bot) SendMessage(message, channel, thread string, whichJanet string) {

	//b.Config.Log.Info("sending message as")

	if whichJanet == "badJanet" {
		//b.Config.Log.Info(whichJanet)

		msg := b.Config.BadJanetSlack.NewOutgoingMessage(message, channel)
		msg.ThreadTimestamp = thread
		b.Config.BadJanetSlack.SendMessage(msg)

		appendMessage := appendQuoteToMessage()
		if appendMessage {
			msg := b.Config.BadJanetSlack.NewOutgoingMessage(badJanetQuote(), channel)
			msg.ThreadTimestamp = thread
			b.Config.BadJanetSlack.SendMessage(msg)
		}
	} else {
		//b.Config.Log.Info("good janet")

		msg := b.Config.Slack.NewOutgoingMessage(message, channel)
		msg.ThreadTimestamp = thread
		b.Config.Slack.SendMessage(msg)
		appendMessage := appendQuoteToMessage()
		if appendMessage {
			msg := b.Config.Slack.NewOutgoingMessage(goodJanetQuote(), channel)
			msg.ThreadTimestamp = thread
			b.Config.Slack.SendMessage(msg)
		}
	}
}

// DMUser sends a message directly to a Slack user.
func (b *Bot) DMUser(message, user string, thread string, whichJanet string) {
	_, _, channel, err := b.Config.Slack.OpenIMChannel(user)

	if whichJanet == "badJanet" {
		_, _, channel, err = b.Config.BadJanetSlack.OpenIMChannel(user)
	}

	if err != nil {
		b.Config.Log.Err(err).KV("user", user).Error("could not open IM channel with user")
		return
	}

	b.SendMessage(message, channel, thread, whichJanet)
}

func (b *Bot) handleError(err error, channel, thread string) bool {
	if err == nil {
		return false
	}

	b.Config.Log.Err(err).Error("error")
	if channel != "" {
		var message string
		if b.Config.Debug {
			message = err.Error()
		} else {
			message = "hi, guys, i'm broken."
		}

		b.SendMessage(message, channel, thread, "")
	}

	return true
}

func (b *Bot) handleReactionAddedEvent(ev *slack.ReactionAddedEvent) {
	if !b.Config.Reactji.Enabled {
		return
	}

	var (
		points int
		reason string
	)
	switch {
	case b.Config.Reactji.Upvote.Contains(ev.Reaction):
		points = +1
	case b.Config.Reactji.Downvote.Contains(ev.Reaction):
		points = -1
	default:
		return
	}

	reason = fmt.Sprintf("adding a :%s: reactji", ev.Reaction)
	b.handleReactionEvent(ev.User, ev.ItemUser, reason, points)
}

func (b *Bot) handleReactionRemovedEvent(ev *slack.ReactionRemovedEvent) {
	if !b.Config.Reactji.Enabled {
		return
	}

	var (
		points int
		reason string
	)
	switch {
	case b.Config.Reactji.Upvote.Contains(ev.Reaction):
		points = -1
	case b.Config.Reactji.Downvote.Contains(ev.Reaction):
		points = +1
	default:
		return
	}

	reason = fmt.Sprintf("removing a :%s: reactji", ev.Reaction)
	b.handleReactionEvent(ev.User, ev.ItemUser, reason, points)
}

func (b *Bot) handleReactionEvent(fromID, toID, reason string, points int) {
	from, err := b.getUserNameByID(fromID)
	if b.handleError(err, "", "") {
		return
	}
	to, err := b.getUserNameByID(toID)
	if b.handleError(err, "", "") {
		return
	}
	from, to = strings.ToLower(from), strings.ToLower(to)

	record := &database.Points{
		From:   from,
		To:     to,
		Points: points,
		Reason: reason,
	}

	err = b.Config.DB.InsertPoints(record)
	if b.handleError(err, "", "") {
		return
	}

	pointsMsg, err := b.getUserPointsMessage(to, reason, points)
	if b.handleError(err, "", "") {
		return
	}

	whichJanet := ""

	if points < 0 {
		whichJanet = "badJanet"
	} else {
		whichJanet = "goodJanet"
	}

	b.DMUser(pointsMsg, fromID, "", whichJanet)
}

func (b *Bot) handleMessageEvent(ev *slack.MessageEvent) {
	if ev.Type != "message" {
		return
	}

	// convert motivates into janet syntax
	if b.Config.Motivate {
		if match := regexps.Motivate.FindStringSubmatch(ev.Text); len(match) > 0 {
			ev.Text = match[1] + "++ for doing good work"
		}
	}

	//b.Config.Log.Info(ev.Text)

	re := regexp.MustCompile("(<@[A-Za-z0-9]+>([\\+]{2,})?([\\-]{2,})?)")
	splits := re.FindAllString(ev.Text, -1)

	if splits != nil {
		for _, split := range splits {

      splitText := split

			switch {
				case regexps.GivePoints.MatchString(splitText):
				b.applyPoints(ev, "", splitText)

				case regexps.TakePoints.MatchString(splitText):
				whichJanet := "badJanet"
				b.applyPoints(ev, whichJanet, splitText)

				case regexps.Throwback.MatchString(ev.Text):
				b.getThrowback(ev)

				case regexps.QueryPoints.MatchString(ev.Text):
				b.queryPoints(ev)
			}
		}
	} else {

		switch {
		case regexps.URL.MatchString(ev.Text):
			b.printURL(ev)
		case regexps.Leaderboard.MatchString(ev.Text):
			b.printLeaderboard(ev)
		}

	}
}

func (b *Bot) printURL(ev *slack.MessageEvent) {
	url, err := b.Config.UI.GetURL("/")
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}

	// ui is disabled
	if url == "" {
		return
	}

	b.SendMessage(url, ev.Channel, ev.ThreadTimestamp, "")
}

func (b *Bot) applyPoints(ev *slack.MessageEvent, whichJanet string, splitText string) {

	b.Config.Log.Info(whichJanet)

	match := regexps.GivePoints.FindStringSubmatch(splitText)
	if len(match) == 0 {
		match = regexps.TakePoints.FindStringSubmatch(splitText)
	}
	if len(match) == 0 {
		return
	}

	// forgive me
	if match[1] != "" {
		// we matched the first alt expression
		match = match[:4]
	} else {
		// we matched the second alt expression
		match = append(match[:1], match[4:]...)
	}

	from, err := b.getUserNameByID(ev.User)
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}
	to, err := b.parseUser(match[1])
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}
	to = strings.ToLower(to)

	if _, blacklisted := b.Config.UserBlacklist[to]; blacklisted {
		b.Config.Log.KV("user", to).Info("user is blacklisted, ignoring karma command")
		return
	}

	points := min(len(match[2])-1, b.Config.MaxPoints)
	if match[2][0] == '-' {
		points *= -1
	}
	reason := match[3]

	if !b.Config.SelfPoints && from == to {
		b.SendMessage("You cannot give yourself points.", ev.Channel, ev.ThreadTimestamp, whichJanet)
		return
	}

	record := &database.Points{
		From:   from,
		To:     to,
		Points: points,
		Reason: reason,
	}

	err = b.Config.DB.InsertPoints(record)
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}

	pointsMsg, err := b.getUserPointsMessage(to, reason, points)
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}

	b.Config.Log.Info("points applied")

	b.SendMessage(pointsMsg, ev.Channel, ev.ThreadTimestamp, whichJanet)
}

func (b *Bot) getThrowback(ev *slack.MessageEvent) {
	match := regexps.Throwback.FindStringSubmatch(ev.Text)
	if len(match) == 0 {
		return
	}

	var (
		user string
		err  error
	)
	if match[1] != "" {
		user, err = b.parseUser(match[1])
		if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
			return
		}
		user = strings.ToLower(user)
	} else {
		user, err = b.getUserNameByID(ev.User)
		if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
			return
		}
	}

	throwback, err := b.Config.DB.GetThrowback(user)
	if err == database.ErrNoSuchUser {
		b.SendMessage(fmt.Sprintf("could not find any karma operations for %s", user), ev.Channel, ev.ThreadTimestamp, "")
		return
	}

	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}

	date := humanize.Time(throwback.Timestamp)
	if throwback.Reason != "" {
		throwback.Reason = fmt.Sprintf(" for %s", throwback.Reason)
	}
	text := fmt.Sprintf("%s received %d points from %s %s%s", munge.Munge(throwback.To), throwback.Points.Points, munge.Munge(throwback.From), date, throwback.Reason)

	b.SendMessage(text, ev.Channel, ev.ThreadTimestamp, "")
}

func (b *Bot) getUserPointsMessage(name, reason string, points int) (string, error) {
	user, err := b.Config.DB.GetUser(name)
	if err != nil {
		return "", err
	}

	text := fmt.Sprintf("%s now has %d points(", name, user.Points)

	if points > 0 {
		text += "+"
	}
	text = fmt.Sprintf("%s%d", text, points)

	if reason != "" {
		text += fmt.Sprintf(" for %s", reason)
	}
	text += ")"

	return text, nil
}

func (b *Bot) printLeaderboard(ev *slack.MessageEvent) {
	match := regexps.Leaderboard.FindStringSubmatch(ev.Text)
	if len(match) == 0 {
		return
	}

	limit := b.Config.LeaderboardLimit
	if match[1] != "" {
		var err error
		limit, err = strconv.Atoi(match[1])
		if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
			return
		}
	}

	text := fmt.Sprintf("*top %d leaderboard*\n", limit)

	url, err := b.Config.UI.GetURL(fmt.Sprintf("/leaderboard/%d", limit))
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}
	if url != "" {
		text = fmt.Sprintf("%s%s\n", text, url)
	}

	leaderboard, err := b.Config.DB.GetLeaderboard(limit)
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}

	for i, user := range leaderboard {
		text += fmt.Sprintf("%d. %s == %d\n", i+1, munge.Munge(user.Name), user.Points)
	}

	b.SendMessage(text, ev.Channel, ev.ThreadTimestamp, "")
}

func (b *Bot) parseUser(user string) (string, error) {
	if match := regexps.SlackUser.FindStringSubmatch(user); len(match) > 0 {
		var err error
		user, err = b.getUserNameByID(match[1])
		if err != nil {
			return "", err
		}
	}

	// check if it is aliased
	if alias, ok := b.Config.Aliases[user]; ok {
		user = alias
	}

	return user, nil
}

func (b *Bot) getUserNameByID(id string) (string, error) {
	userInfo, err := b.Config.Slack.GetUserInfo(id)
	if err != nil {
		return "", err
	}

	return userInfo.Name, nil
}

func (b *Bot) queryPoints(ev *slack.MessageEvent) {
	match := regexps.QueryPoints.FindStringSubmatch(ev.Text)
	if len(match) == 0 {
		return
	}

	name, err := b.parseUser(match[1])
	if b.handleError(err, ev.Channel, ev.ThreadTimestamp) {
		return
	}
	name = strings.ToLower(name)

	user, err := b.Config.DB.GetUser(name)
	switch {
	case err == database.ErrNoSuchUser:
		// override debug mode
		b.SendMessage(err.Error(), ev.Channel, ev.ThreadTimestamp, "")
	case b.handleError(err, ev.Channel, ev.ThreadTimestamp):
	default:
		b.SendMessage(fmt.Sprintf("%s == %d", user.Name, user.Points), ev.Channel, ev.ThreadTimestamp, "")
	}
}
