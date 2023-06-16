package main

import (
	"reflect"
	"testing"

	"gopkg.in/irc.v3"
)

func Test_messageOutcome(t *testing.T) {
	tests := []struct {
		name string
		m    string
		want outcome
	}{
		{
			"nothing",
			"@color=#FF69B4;mod=0;user-type;badges;first-msg=0;returning-chatter=0;turbo=0;badge-info;flags :snip!snip@snip.tmi.twitch.tv PRIVMSG #eznoprimes :sample message",
			outcome{},
		},
		{
			"non-mod !nonprimesubcount - doesn't work",
			"@color=#FF69B4;mod=0;user-type;badges;first-msg=0;returning-chatter=0;turbo=0;badge-info;flags :snip!snip@snip.tmi.twitch.tv PRIVMSG #eznoprimes :!nonprimesubcount 3000",
			outcome{},
		},

		{
			"mod !nonprimesubcount",
			"@emotes;first-msg=0;badge-info=subscriber/5;badges=moderator/1,subscriber/3,hype-train/1;flags;subscriber=1;mod=1;turbo=0;color=#001122;returning-chatter=0;user-type=mod :snip!snip@snip.tmi.twitch.tv PRIVMSG #eznoprimes :!nonprimesubcount 123",
			outcome{
				overwriteSubs: true,
				subs:          123,
			},
		},
		{
			"broadcaster !nonprimesubcount",
			"@badges=broadcaster/1,subscriber/3018,partner/1;returning-chatter=0;subscriber=1;turbo=0;badge-info=subscriber/18;color=#FFC2E5;flags;mod=0;user-type;emotes;first-msg=0;tmi-sent-ts=1682991722107 :eznoprimes!eznoprimes@eznoprimes.tmi.twitch.tv PRIVMSG #eznoprimes :!nonprimesubcount 123",
			outcome{
				overwriteSubs: true,
				subs:          123,
			},
		},

		{
			"gifted sub - doesn't count for partner plus",
			"@msg-param-months=5;msg-param-recipient-user-name=snip;mod=0;msg-param-sub-plan-name=Channel\\sSubscription\\s(eznoprimes);system-msg=snip\\sgifted\\sa\\sTier\\s1\\ssub\\sto\\ssnip!;color=#001122;flags;msg-param-sub-plan=1000;emotes;msg-id=subgift;login=snip;msg-param-gift-months=1;msg-param-recipient-msg-param-sender-count=0;badge-info;badges=premium/1;user-type;subscriber=0; :tmi.twitch.tv USERNOTICE #eznoprimes",
			outcome{
				incrementSubs: false,
				subs:          0,
			},
		},
		{
			"prime resub - doesn't count for partner plus",
			"@login=snip;user-type;badge-info=subscriber/2;flags;msg-param-cumulative-months=2;msg-param-multimonth-duration=0;msg-param-was-gifted=false;emotes;mod=0;msg-id=resub;msg-param-multimonth-tenure=0;msg-param-should-share-streak=0;msg-param-sub-plan-name=Channel\\sSubscription\\s(eznoprimes);msg-param-sub-plan=Prime;badges=subscriber/2,premium/1;system-msg=snip\\ssubscribed\\swith\\sPrime.\\sThey've\\ssubscribed\\sfor\\s2\\smonths!;subscriber=1;msg-param-months=0;color=#001122 :tmi.twitch.tv USERNOTICE #eznoprimes :sample sub text",
			outcome{
				incrementSubs: false,
				subs:          0,
			},
		},
		{
			"T1 resub",
			"@login=snip;user-type;badge-info=subscriber/2;flags;msg-param-cumulative-months=2;msg-param-multimonth-duration=0;msg-param-was-gifted=false;emotes;mod=0;msg-id=resub;msg-param-multimonth-tenure=0;msg-param-should-share-streak=0;msg-param-sub-plan-name=Channel\\sSubscription\\s(eznoprimes);msg-param-sub-plan=1000;badges=subscriber/2,premium/1;system-msg=snip\\ssubscribed\\swith\\sPrime.\\sThey've\\ssubscribed\\sfor\\s2\\smonths!;subscriber=1;msg-param-months=0;color=#001122 :tmi.twitch.tv USERNOTICE #eznoprimes :sample sub text",
			outcome{
				incrementSubs: true,
				subs:          1,
			},
		},
		{
			"prime sub - doesn't count for partner plus",
			"@login=snip;user-type;badge-info=subscriber/2;flags;msg-param-cumulative-months=2;msg-param-multimonth-duration=0;msg-param-was-gifted=false;emotes;mod=0;msg-id=sub;msg-param-multimonth-tenure=0;msg-param-should-share-streak=0;msg-param-sub-plan-name=Channel\\sSubscription\\s(eznoprimes);msg-param-sub-plan=Prime;badges=subscriber/2,premium/1;system-msg=snip\\ssubscribed\\swith\\sPrime.\\sThey've\\ssubscribed\\sfor\\s2\\smonths!;subscriber=1;msg-param-months=0;color=#001122 :tmi.twitch.tv USERNOTICE #eznoprimes :sample sub text",
			outcome{
				incrementSubs: false,
				subs:          0,
			},
		},
		{
			"T1 sub",
			"@login=snip;user-type;badge-info=subscriber/2;flags;msg-param-cumulative-months=2;msg-param-multimonth-duration=0;msg-param-was-gifted=false;emotes;mod=0;msg-id=sub;msg-param-multimonth-tenure=0;msg-param-should-share-streak=0;msg-param-sub-plan-name=Channel\\sSubscription\\s(eznoprimes);msg-param-sub-plan=1000;badges=subscriber/2,premium/1;system-msg=snip\\ssubscribed\\swith\\sPrime.\\sThey've\\ssubscribed\\sfor\\s2\\smonths!;subscriber=1;msg-param-months=0;color=#001122 :tmi.twitch.tv USERNOTICE #eznoprimes :sample sub text",
			outcome{
				incrementSubs: true,
				subs:          1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messageOutcome(irc.MustParseMessage(tt.m)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("messageOutcome() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_state_MergeOutcome(t *testing.T) {
	tests := []struct {
		name     string
		initial  state
		applied  outcome
		expected action
		final    state
	}{
		{
			"do nothing",
			state{
				subs: 1234,
			},
			outcome{},
			action{},
			state{
				subs: 1234,
			},
		},

		{
			"increment subs",
			state{
				subs: 1234,
			},
			outcome{
				incrementSubs: true,
				subs:          1,
			},
			action{
				writeSubs: true,
			},
			state{
				subs: 1235,
			},
		},

		{
			"overwrite subs",
			state{
				subs: 1234,
			},
			outcome{
				overwriteSubs: true,
				subs:          69,
			},
			action{
				writeSubs: true,
			},
			state{
				subs: 69,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initial
			if got := state.MergeOutcome(tt.applied); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("state.MergeOutcome() actions = %v, want %v", got, tt.expected)
			}
			if !reflect.DeepEqual(state, tt.final) {
				t.Errorf("state.MergeOutcome() final = %v, want %v", state, tt.final)
			}
		})
	}
}
