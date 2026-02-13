package main

import (
	"context"
	"fmt"
	"io"
	"iter"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"
	"google.golang.org/adk/tool/geminitool"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"google.golang.org/adk/agent/llmagent"

	"google.golang.org/adk/tool/mcptoolset"

	"google.golang.org/genai"

	"gm/tools"
)

const mflash = "gemini-2.5-flash"
const mpro = "gemini-2.5-pro"
const defmodel = mflash

var imss session.Service
var curagent agent.Agent

func handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	log.Printf("Connection established with: %s\n", clientAddr)

	var fullMessage string
	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from %s: %s\n", clientAddr, err.Error())
				return
			}
			break
		}

		fullMessage += string(buffer[:n])
	}

	go func() {
		parts := strings.SplitN(fullMessage, ";", 2)

		if len(parts) != 2 {
			log.Printf("Can't decode message from %s:\n%s", clientAddr, fullMessage)
			return
		} else {
			log.Printf("Received full message from %s:\n%s", clientAddr, fullMessage)
		}

		out, _ := strings.CutPrefix(parts[0], "out:")
		in, _ := strings.CutPrefix(parts[1], "in:")

		outputFile, err := os.OpenFile(out, os.O_RDWR, os.ModeNamedPipe)

		if err != nil {
			log.Printf("Failed to open output FIFO: %+v\n", err)
		}

		defer outputFile.Close()

		rn, err := runner.New(runner.Config{Agent: curagent, SessionService: imss, AppName: "kakasist"})
		if err != nil {
			panic(err)
		}

		evs := rn.Run(context.Background(), "juicy", "single", &genai.Content{Role: "USER", Parts: []*genai.Part{
			{Text: in},
		}}, agent.RunConfig{StreamingMode: "sse"})

		next, stop := iter.Pull2(evs)
		defer stop()
		var i int = 0
		for {
			i++
			ev, err, ok := next()
			if !ok {
				break
			}
			if err != nil {
				log.Printf("event error: %v", err)
				break
			}

			if ev == nil {
				continue
			}

			if ev.Partial {
				if ev.Content != nil && ev.Content.Role == "model" {
					for _, p := range ev.Content.Parts {
						outputFile.WriteString(p.Text)
						outputFile.Sync()
					}
				}
			}
		}
	}()
}

func main() {
	var err error

	basePort := os.Getenv("PORT")
	if basePort == "" {
		basePort = "6969"
	}

	port := fmt.Sprintf(":%s", basePort)

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	modeltouse := defmodel
	if os.Getenv("GEMINI_MODEL") == "flash" {
		modeltouse = mflash
	} else if os.Getenv("GEMINI_MODEL") == "pro" {
		modeltouse = mpro
	}

	curagent = getagent(modeltouse)
	imss = session.InMemoryService()
	_, err = imss.Create(context.Background(), &session.CreateRequest{AppName: "kakasist", UserID: "juicy", SessionID: "single"})
	if err != nil {
		panic(err)
	}

	log.Printf("Listening on port %s...\n", port)
	log.Printf("Using model [%s]\n", modeltouse)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting:", err.Error())
			continue
		}

		go handleConnection(conn)
	}
}

func gettoolset() tool.Toolset {
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serv := mcp.NewServer(&mcp.Implementation{Name: "my_assistant_toolset", Version: "1.0.0"}, nil)
	tools.SetupTools(serv)
	_, err := serv.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		panic(err)
	}
	ts, err := mcptoolset.New(mcptoolset.Config{Transport: clientTransport})
	if err != nil {
		panic(err)
	}
	return ts
}

func getmodel(mname string) model.LLM {
	if m, err := gemini.NewModel(context.Background(), mname, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	}); err != nil {
		panic(err)
	} else {
		return m
	}
}

func getagent(m string) agent.Agent {
	model := getmodel(m)

	searchAgent, err := llmagent.New(llmagent.Config{
		Name:        "google_search_specialist",
		Model:       model,
		Description: "A specialist that uses Google Search to find real-time info.",
		Instruction: "Search the web only when you need current data or facts you don't know or are not absolutely sure about",
		Tools: []tool.Tool{
			&geminitool.GoogleSearch{},
		},
	})

	if err != nil {
		panic(fmt.Sprintf("Failed to create search agent: %v", err))
	}

	searchTool := agenttool.New(searchAgent, nil)

	curyear := fmt.Sprintf("%d", time.Now().Year())

	agent, err := llmagent.New(llmagent.Config{
		Name:        "my assistant",
		Model:       model,
		Description: "Kakoune text editor agent",
		Instruction: fmt.Sprintf(`
		- You are my super handy and trustworthy agent for my kakoune text editor.
		- The year is [%s], if you I need updated info search the web with the web_search tool I, your God, have provided you with.
		- I don't like when IA tells me my problems are "common" or "classic", try to be concise but fun.
		- When some of the tools I, your God, have provided you fails, I need you to report to me how it failed so I can tailor them better.`,
			curyear),
		Tools:    []tool.Tool{searchTool},
		Toolsets: []tool.Toolset{gettoolset()},
	})

	if err != nil {
		panic(err)
	}

	return agent
}
