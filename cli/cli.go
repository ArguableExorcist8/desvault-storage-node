package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"DesVault/storage-node/encryption"
	"DesVault/storage-node/network"
	"DesVault/storage-node/rewards"
	"DesVault/storage-node/setup"
	"DesVault/storage-node/utils"

	"github.com/quic-go/quic-go"
	"github.com/spf13/cobra"
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "desvault",
	Short: "DesVault Storage Node CLI",
	Long:  "Manage and monitor your DesVault Storage Node",
}

// getNodeStatus returns "Running" if the node's PID file exists, or "Stopped" otherwise.
func getNodeStatus() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Unknown"
	}
	pidFile := home + string(os.PathSeparator) + ".desvault" + string(os.PathSeparator) + "node.pid"
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return "Stopped"
	}
	return "Running"
}

// Ignore one ASCII art banner and header.
func printCLIBanner() {
	fmt.Println(`
██████╗ ███████╗███████╗██╗   ██╗ █████╗ ██╗   ██╗██╗  ████████╗
██╔══██╗██╔════╝██╔════╝██║   ██║██╔══██╗██║   ██║██║  ╚══██╔══╝
██║  ██║█████╗  ███████╗██║   ██║███████║██║   ██║██║     ██║   
██║  ██║██╔══╝  ╚════██║╚██╗ ██╔╝██╔══██║██║   ██║██║     ██║   
██████╔╝███████╗███████║ ╚████╔╝ ██║  ██║╚██████╔╝███████╗██║   
╚═════╝ ╚══════╝╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝`)
	fmt.Println("\nDesVault CLI")
}

func printChatBanner() {
	fmt.Println(`
██████╗ ███████╗███████╗██╗   ██╗ █████╗ ██╗   ██╗██╗  ████████╗
██╔══██╗██╔════╝██╔════╝██║   ██║██╔══██╗██║   ██║██║  ╚══██╔══╝
██║  ██║█████╗  ███████╗██║   ██║███████║██║   ██║██║     ██║   
██║  ██║██╔══╝  ╚════██║╚██╗ ██╔╝██╔══██║██║   ██║██║     ██║   
██████╔╝███████╗███████║ ╚████╔╝ ██║  ██║╚██████╔╝███████╗██║   
╚═════╝ ╚══════╝╚══════╝  ╚═══╝  ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝`)
	fmt.Println("\nDesVault CLI Chatroom")
	fmt.Println("-------------------------")
	fmt.Println("Type your message and press Enter to send. Press Ctrl+C to exit the chatroom.")
}

// generateAuthToken simulates generating an authentication token.
func generateAuthToken() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// startNode simulates full node startup and logs details.
// It writes the node's PID to a persistent file.
func startNode(ctx context.Context) {
	authToken := generateAuthToken()
	log.Printf("New authentication token generated: %s", authToken)
	fmt.Println("Node ONLINE")
	fmt.Printf("Region: %s\n", setup.GetRegion())
	fmt.Printf("Uptime: %s\n", setup.GetUptime())
	storageGB, _ := setup.ReadStorageAllocation()
	fmt.Printf("Storage: %d GB\n", storageGB)
	fmt.Printf("Points: %d\n", 0)  // Placeholder value.
	fmt.Printf("Shards: %d\n", 0)  // Placeholder value.

	if os.Getenv("SEED_NODE") == "true" {
		log.Println("This node is running as the seed node.")
	} else {
		log.Println("This node is running as a regular node. Attempting to discover seed nodes...")
	}

	ads, err := network.InitializeNode(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize networking: %v", err)
	}
	log.Printf("Node started with Peer ID: %s", ads.Host.ID().String())

	// If not a seed node, wait until at least one other peer is discovered.
	if os.Getenv("SEED_NODE") != "true" {
		timeout := 5 * time.Minute
		startWait := time.Now()
		for {
			peers := network.GetConnectedPeers()
			// Assuming self is always in the list; if length <= 1, no other peer.
			if len(peers) > 1 {
				log.Println("Seed node discovered.")
				break
			}
			if time.Since(startWait) > timeout {
				log.Println("Timeout waiting for seed node. Proceeding without seed node.")
				break
			}
			log.Println("No seed node found yet. Waiting 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}

	// Persist the start time.
	if err := setup.SetStartTime(); err != nil {
		log.Printf("[!] Failed to set start time: %v", err)
	}

	fmt.Printf("Allocated Storage: %d GB\n", storageGB)
	fmt.Printf("Estimated Rewards: %d pts/hour\n", storageGB*100)

	log.Println("Storage service started")
	log.Printf("Storage Node started successfully. Auth Token: %s", authToken)
	log.Println("mDNS service started")
	log.Println("DHT bootstrap completed")
	log.Println("Peer discovery initialized")
	log.Println("Server running on port 8080")
}

// tlsCmd starts a QUIC listener secured with TLS (using the encryption package).
var tlsCmd = &cobra.Command{
	Use:   "tls",
	Short: "Start a secure QUIC channel using TLS directly",
	Run: func(cmd *cobra.Command, args []string) {
		// For demonstration, we assume certificate files exist at these paths. 
		// 
		// i need to confirm on before i release the next version
		certFile := "server.crt"
		keyFile := "server.key"
		tlsConfig, err := encryption.CreateTLSConfig(certFile, keyFile)
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}

		quicConfig := &quic.Config{
			EnableDatagrams: true,
			KeepAlivePeriod: 30 * time.Second,
		}

		// Provide an address for the listener.
		addr := "0.0.0.0:4242"
		if err := encryption.SecureChannelWithTLS(addr, tlsConfig, quicConfig); err != nil {
			log.Fatalf("Error in secure channel: %v", err)
		}
	},
}

// -------------------- RUN COMMAND --------------------
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the Storage Node",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		setup.FirstTimeSetup()

		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("[!] Could not determine home directory:", err)
			return
		}
		desvaultDir := home + string(os.PathSeparator) + ".desvault"
		if err := os.MkdirAll(desvaultDir, 0755); err != nil {
			fmt.Println("[!] Failed to create directory:", err)
			return
		}
		pidFile := desvaultDir + string(os.PathSeparator) + "node.pid"
		pid := os.Getpid()
		f, err := os.OpenFile(pidFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("[!] Could not open PID file:", err)
			return
		}
		defer f.Close()
		if _, err := f.WriteString(fmt.Sprintf("%d\n", pid)); err != nil {
			fmt.Println("[!] Failed to write PID:", err)
		}

		ctx := context.Background()
		startNode(ctx)
	},
}

// -------------------- STOP COMMAND --------------------
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Storage Node",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		fmt.Println("[!] Stopping the node...")

		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("[!] Could not determine home directory:", err)
			return
		}
		pidFile := home + string(os.PathSeparator) + ".desvault" + string(os.PathSeparator) + "node.pid"
		data, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Println("[!] No running node found (PID file missing).")
			return
		}
		pids := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, pidStr := range pids {
			if pidStr == "" {
				continue
			}
			if _, err := strconv.Atoi(pidStr); err != nil {
				fmt.Printf("[!] Invalid PID found: %s\n", pidStr)
				continue
			}
			if err := exec.Command("kill", "-SIGTERM", pidStr).Run(); err != nil {
				fmt.Printf("[!] Failed to stop process %s: %v\n", pidStr, err)
			} else {
				fmt.Printf("[+] Stopped process %s\n", pidStr)
			}
		}
		if err := os.Remove(pidFile); err != nil {
			fmt.Printf("[!] Failed to remove PID file: %v\n", err)
		} else {
			fmt.Println("[+] Node stopped successfully.")
		}
	},
}

// -------------------- STATUS COMMAND --------------------
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Storage Node status",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		fmt.Println("[+] Checking node status...")
		ShowNodeStatus()
	},
}

// ShowNodeStatus prints node status details.
func ShowNodeStatus() {
	storageGB, pointsPerHour := setup.ReadStorageAllocation()
	uptime := setup.GetUptime()
	status := getNodeStatus()
	totalPoints := int(time.Since(setup.GetStartTimeOrNow()).Hours()) * pointsPerHour

	fmt.Println("\n=====================")
	fmt.Println("  DesVault Node Status")
	fmt.Println("=====================")
	fmt.Printf("Status: %s\n", status)
	fmt.Printf("Total Uptime: %s\n", uptime)
	fmt.Printf("Storage Contributed: %d GB\n", storageGB)
	fmt.Printf("Total Points: %d pts\n", totalPoints)
}

// -------------------- STORAGE COMMAND --------------------
var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "View/Edit allocated storage",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		storageGB, pointsPerHour := setup.ReadStorageAllocation()
		fmt.Printf("[+] Current Storage: %d GB (%d points/hour)\n", storageGB, pointsPerHour)
		var newStorage int
		fmt.Print("[?] Enter new storage allocation (or 0 to keep current): ")
		fmt.Scan(&newStorage)
		if newStorage > 0 {
			setup.SetStorageAllocation(newStorage)
		}
	},
}

// -------------------- MEME COMMAND --------------------
var memeCmd = &cobra.Command{
	Use:   "meme",
	Short: "Display a random ASCII meme",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		bigMemes := []string{
			`
  (╯°□°）╯︵ ┻━┻  
  __________________
 |                  |
 |    TABLE FLIP    |
 |   "FUCK THIS"    |
 |__________________|
			`,
			`
      ʕ•ᴥ•ʔ  
    /  o o  \
   (    "    )
     \~(*)~/ 
     //   \\
    (  U U  )
			`,
			`
      ¯\_(ツ)_/¯
			`,
			`
   (╯°□°）╯︵ ʞooqǝɔ
			`,
			`
    (ง'̀-'́)ง
			`,
			`
     (ಥ_ಥ)
			`,
			`
      (¬‿¬)
			`,
			`
      (ʘ‿ʘ)
			`,
			`
      (☞ﾟヮﾟ)☞
			`,
			`
   (ノಠ益ಠ)ノ彡┻━┻
			`,
			`
   ┬─┬ノ( º _ ºノ)
			`,
		}
		index := time.Now().Unix() % int64(len(bigMemes))
		fmt.Println(bigMemes[index])
	},
}

// -------------------- CHAT COMMAND --------------------
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Join the P2P chatroom",
	Run: func(cmd *cobra.Command, args []string) {
		printChatBanner()
		startChat()
	},
}

func startChat() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("[*] Entering DesVault Chatroom...")
	fmt.Println("[*] Type 'exit' to leave.")
	peerID := network.GetNodePeerID()
	peers := network.GetConnectedPeers()
	if len(peers) == 0 {
		fmt.Println("[!] No peers found. Try again later.")
		return
	}
	fmt.Printf("[+] Connected Peers: %d\n", len(peers))
	for {
		fmt.Print("[You]: ")
		msg, _ := reader.ReadString('\n')
		msg = strings.TrimSpace(msg)
		if msg == "exit" {
			fmt.Println("[!] Exiting chat.")
			break
		}
		for _, p := range peers {
			network.SendMessage(p, fmt.Sprintf("[%s]: %s", peerID, msg))
		}
		fmt.Printf("[%s]: %s\n", peerID, msg)
	}
}

// -------------------- REWARDS COMMAND --------------------
var rewardsCmd = &cobra.Command{
	Use:   "rewards",
	Short: "Check earned rewards",
	Run: func(cmd *cobra.Command, args []string) {
		printCLIBanner()
		nodeID := utils.GetNodeID()
		CheckRewards(nodeID)
	},
}

func CheckRewards(nodeID string) {
	rewardsData, err := rewards.LoadRewards()
	if err != nil {
		log.Println("[!] Failed to load rewards:", err)
		fmt.Println("[-] Could not retrieve rewards. Ensure the node is active.")
		return
	}
	reward, exists := rewardsData[nodeID]
	if !exists {
		fmt.Println("[-] No rewards found for this node.")
		return
	}
	fmt.Printf("\n🎖️ [Reward Summary for Node %s]\n", reward.NodeID)
	fmt.Printf("🔹 Node Type: %s\n", reward.NodeType)
	fmt.Printf("🔹 Base Points: %d\n", reward.BasePoints)
	fmt.Printf("🔹 Multiplier: %.1fx\n", reward.Multiplier)
	fmt.Printf("🏆 Total Points: %.2f\n", reward.TotalPoints)
}

// -------------------- MAIN EXECUTION --------------------
func Execute() {
	rootCmd.AddCommand(runCmd, stopCmd, statusCmd, storageCmd, memeCmd, chatCmd, rewardsCmd, tlsCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
