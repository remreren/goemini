package protocol

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	keyFile   string
	certFile  string
	servePath string
)

var rootCmd = &cobra.Command{
	Use:   "goemini",
	Short: "Static gemini server.",
	Long:  `A static server uses gemini protocol to serve gemini files.`,
	Run: func(cmd *cobra.Command, args []string) {
		_start()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&servePath, "path", "p", "./", "The path to serve the files.")
	rootCmd.Flags().StringVarP(&certFile, "cert", "c", "gemini.cert", "The path to the certificate file.")
	rootCmd.Flags().StringVarP(&keyFile, "key", "k", "gemini.key", "The path to the key file.")
}

func _start() {
	mime.AddExtensionType(".gmi", "text/gemini")
	mime.AddExtensionType(".gmni", "text/gemini")
	mime.AddExtensionType(".gemini", "text/gemini")
	mime.AddExtensionType(".md", "text/markdown")

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Failed to load key pair: %v", err)
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}

	listener, err := tls.Listen("tcp", ":1965", config)
	if err != nil {
		log.Fatalf("Failed to listen on port 1965: %v", err)
	}
	defer listener.Close()

	log.Println("Gemini server is running on port 1965")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func readFile(path string) ([]byte, string, error) {
	// merge "servePath" with the path we got from last step
	// Example1: ./ + index.gmi -> ./index.gmi
	// Example2: ./ + other/page.gmi -> ./other/page.gmi
	path = filepath.Join(servePath, path)

	// guess mime type based on file extension
	mimetype := mime.TypeByExtension(filepath.Ext(path))

	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return nil, "", err
	}
	defer file.Close()

	br := bufio.NewReader(file)

	buffer := make([]byte, 4096)
	content := make([]byte, 0)

	for {

		b, err := br.Read(buffer)

		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Println(err)
			break
		}

		if err != nil {
			break
		}

		content = append(content, buffer[:b]...)
	}

	return content, mimetype, nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Failed to read from connection: %v", err)
		return
	}

	request := string(buf[:n])
	log.Printf("Received request: %s", request)

	path := parseRequestPath(request)

	// if path has suffix "/" or empty, then append "index.gmi" to it
	if path == "" || strings.HasSuffix(path, "/") {
		path += "index.gmi"
	}

	content, mime, err := readFile(path)
	if err != nil {
		sendResponse(conn, "51 Not found\r\n")
		return
	}

	response := fmt.Sprintf("20 %s\r\n%s", mime, content)

	sendResponse(conn, response)
}

func parseRequestPath(request string) string {
	request = strings.TrimSpace(request)
	if !strings.HasPrefix(request, "gemini://") {
		return ""
	}

	// Remove the "gemini://" prefix
	// Example: gemini://localhost/index.gmi -> localhost/index.gmi
	request = strings.TrimPrefix(request, "gemini://")

	// Then split the request into domain and path
	// Example1: localhost/index.gmi -> [localhost, index.gmi]
	// Example2: localhost/other/page.gmi -> [localhost, other/page.gmi]
	// Example3: localhost/other/ or localhost/other -> [localhost, other]
	// Discard the domain part and get the path part
	parts := strings.SplitN(request, "/", 2)
	path := parts[1]

	return path
}

func sendResponse(conn net.Conn, response string) {
	_, err := conn.Write([]byte(response))
	if err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}
