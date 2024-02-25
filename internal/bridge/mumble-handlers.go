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
    // Ensure l.Bridge is not nil
    if l.Bridge == nil {
        log.Println("Bridge is nil")
        return
    }

    // Directly lock MumbleUsersMutex without nil check
    l.Bridge.MumbleUsersMutex.Lock()
    defer l.Bridge.MumbleUsersMutex.Unlock()

    // Initialize MumbleUsers map
    l.Bridge.MumbleUsers = make(map[string]bool)

    // Ensure MumbleClient and its nested fields are not nil
    if l.Bridge.MumbleClient == nil || l.Bridge.MumbleClient.Self == nil || l.Bridge.MumbleClient.Self.Channel == nil {
        log.Println("MumbleClient, Self, or Self.Channel is nil")
        return
    }

    for _, user := range l.Bridge.MumbleClient.Self.Channel.Users {
        if user.Name != l.Bridge.MumbleClient.Self.Name {
            l.Bridge.MumbleUsers[user.Name] = true
        }
    }

    // Update metrics, assuming promMumbleUsers is correctly initialized elsewhere
    if promMumbleUsers != nil { // Make sure this is a valid check based on its type
        promMumbleUsers.Set(float64(len(l.Bridge.MumbleUsers)))
    } else {
        log.Println("promMumbleUsers is nil")
    }

    // Update voice targets
    l.updateVoiceTargets()
}



func (l *MumbleListener) updateVoiceTargets() {
    if l.Bridge.MumbleClient == nil || l.Bridge.MumbleClient.VoiceTarget == nil {
        log.Println("MumbleClient or VoiceTarget is nil")
        return
    }
    
    // Clear the current voice target's users
    l.Bridge.MumbleClient.VoiceTarget.Clear()

    // Iterate over all users in the bot's current channel and add them to the voice target
    for _, user := range l.Bridge.MumbleClient.Self.Channel.Users {
        l.Bridge.MumbleClient.VoiceTarget.AddUser(user)
    }
}


func (l *MumbleListener) MumbleConnect(e *gumble.ConnectEvent) {
	l.Bridge.MumbleClient = e.Client
	//join specified channel
	startingChannel := e.Client.Channels.Find(l.Bridge.BridgeConfig.MumbleChannel...)
	if startingChannel != nil {
		e.Client.Self.Move(startingChannel)
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
}
