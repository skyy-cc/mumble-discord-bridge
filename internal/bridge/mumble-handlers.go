package bridge

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/stieneee/gumble/gumble"
)

// MumbleListener Handle mumble events
type MumbleListener struct {
	Bridge *BridgeState
}

func (l *MumbleListener) updateUsers() {
	if l == nil || l.Bridge == nil || l.Bridge.MumbleClient == nil || l.Bridge.MumbleClient.Self == nil || l.Bridge.MumbleClient.Self.Channel == nil {
        log.Println("One or more required properties are nil. Exiting updateUsers early.")
        return
    }

	l.Bridge.MumbleUsersMutex.Lock()
	l.Bridge.MumbleUsers = make(map[string]bool)
	for _, user := range l.Bridge.MumbleClient.Self.Channel.Users {
		//note, this might be too slow for really really big channels?
		//event listeners block while processing
		//also probably bad to rebuild the set every user change.
		if user.Name != l.Bridge.MumbleClient.Self.Name {
			l.Bridge.MumbleUsers[user.Name] = true
		}
	}
	promMumbleUsers.Set(float64(len(l.Bridge.MumbleUsers)))
	l.Bridge.MumbleUsersMutex.Unlock()

}

func (l *MumbleListener) MumbleConnect(e *gumble.ConnectEvent) {
	//join specified channel
	startingChannel := e.Client.Channels.Find(l.Bridge.BridgeConfig.MumbleChannel...)
	if startingChannel != nil {
		e.Client.Self.Move(startingChannel)
		// Add starting channel as a target
		voiceTarget := gumble.VoiceTarget{}
        voiceTarget.ID = 1 // Set an ID for the voice target (IDs 1-31 are valid)

        // Add the channel to the voice target
        voiceTarget.AddChannel(startingChannel, true, true, "")

        // Send the voice target configuration to the server
        e.Client.Send(&voiceTarget)
	}

	// l.updateUsers() // patch below

	// This is an ugly patch Mumble Client state is slow to update
	time.AfterFunc(5*time.Second, func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Failed to mumble user list %v \n", r)
			}
		}()
		l.updateUsers()
	})
}

func (l *MumbleListener) MumbleUserChange(e *gumble.UserChangeEvent) {
	if e == nil {
        log.Println("User change event is nil")
        return
    }
    if e.User == nil {
        log.Println("User data is nil")
        return
    }

	l.updateUsers()

	if e.Type.Has(gumble.UserChangeConnected) {

		log.Println("User connected to mumble " + e.User.Name)

		if !l.Bridge.BridgeConfig.MumbleDisableText {
			e.User.Send("Mumble-Discord-Bridge " + l.Bridge.BridgeConfig.Version)

			// Tell the user who is connected to discord
			l.Bridge.DiscordUsersMutex.Lock()
			if len(l.Bridge.DiscordUsers) == 0 {
				e.User.Send("No users connected to Discord")
			} else {
				s := "Connected to Discord: "

				arr := []string{}
				for u := range l.Bridge.DiscordUsers {
					arr = append(arr, l.Bridge.DiscordUsers[u].username)
				}

				s = s + strings.Join(arr[:], ",")

				e.User.Send(s)
			}
			l.Bridge.DiscordUsersMutex.Unlock()

		}

		// Send discord a notice
		l.Bridge.discordSendMessageAll(e.User.Name + " has joined mumble")
	}

	if e.Type.Has(gumble.UserChangeDisconnected) {
		l.Bridge.discordSendMessageAll(e.User.Name + " has left mumble")
		log.Println("User disconnected from mumble " + e.User.Name)
	}

	if e.Type.Has(gumble.UserChangeChannel) {
        if e.User.Channel == l.Bridge.MumbleClient.Self.Channel {
            log.Println("User started listening to the channel:", e.User.Name)
        } else {
            log.Println("User stopped listening to the channel:", e.User.Name)
        }
    }
}