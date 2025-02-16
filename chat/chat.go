package chat

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p"
)

// StartChat launches a basic P2P chat session.
func StartChat() {
	fmt.Println("[*] Entering DesVault Chatroom...")
	fmt.Println("[*] Type your message and press ENTER to send. Type 'exit' to quit.")

	host, err := libp2p.New()
	if err != nil {
		fmt.Println("[!] Error starting chat node:", err)
		return
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("[You]: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		if text == "exit" {
			fmt.Println("[*] Leaving chatroom...")
			break
		}

		// For demonstration, iterate over peers in the local Peerstore.
		for _, p := range host.Peerstore().Peers() {
			fmt.Printf("[Sent to %s]: %s\n", p.ShortString(), text)
		}
	}
}
