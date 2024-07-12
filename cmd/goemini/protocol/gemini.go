package protocol

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"syscall"

	tls "github.com/secure-for-ai/goktls" // change this via "crypto/tls"

	"github.com/spf13/cobra"
)

var (
	keyFile   string
	certFile  string
	servePath string
	debug     bool
)

var rootCmd = &cobra.Command{
	Use:   "goemini",
	Short: "Static gemini server.",
	Long:  "A static server uses gemini protocol to serve gemini files.",
	Run: func(cmd *cobra.Command, args []string) {
		_start()
	},
}

var (
	ErrBadRequest   = errors.New("bad request")
	ErrFileNotFound = errors.New("file not found")
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&servePath, "path", "p", "./", "The path to serve the files.")
	rootCmd.Flags().StringVarP(&certFile, "cert", "c", "gemini.cert", "The path to the certificate file.")
	rootCmd.Flags().StringVarP(&keyFile, "key", "k", "gemini.key", "The path to the key file.")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode.")
}

func _start() {
	var err error

	servePath, err = filepath.EvalSymlinks(servePath)
	if err != nil {
		log.Fatalf("Failed to evaluate symlinks: %v", err)
	}

	servePath, err = filepath.Abs(servePath)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

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

	if debug {
		f, err := os.Create("cpu.prof")
		if err != nil {
			log.Fatalf("Failed to create cpu profile: %v", err)
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("Failed to start cpu profile: %v", err)
		}

		defer pprof.StopCPUProfile()
	}

	// Channel to listen for interrupt signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Channel to signal the shutdown of the server
	shutdownChan := make(chan struct{})

	go func() {
		<-signalChan
		log.Println("Received interrupt signal, shutting down gracefully...")
		listener.Close()
		close(shutdownChan)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-shutdownChan:
				log.Println("Server shut down gracefully")
				return
			default:
				log.Printf("Failed to accept connection: %v", err)
			}
			continue
		}
		go handleConnection(conn)
	}
}

func sendFile(file *os.File, conn net.Conn) error {
	defer file.Close()

	_, err := io.Copy(conn, file)
	if err != nil {
		return err
	}

	return nil
}

func getFile(path string) (*os.File, string, error) {
	var err error

	path = filepath.Clean(path)
	path = filepath.Join(servePath, path)

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		log.Printf("Failed to evaluate symlinks: %v", err)
		return nil, "", ErrBadRequest
	}

	path, err = filepath.Abs(path)
	if err != nil {
		log.Printf("Failed to get absolute path: %v", err)
		return nil, "", ErrBadRequest
	}

	// check for the traversal attack
	if !strings.HasPrefix(path, servePath) {
		log.Printf("Path is not in the serve path: %s", path)
		return nil, "", errors.New("you naughty hacker :)")
	}

	// guess mime type based on file extension
	mimetype := mime.TypeByExtension(filepath.Ext(path))

	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return nil, "", ErrFileNotFound
	}

	return file, mimetype, nil
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

	path, err := parseRequestPath(request)
	if err != nil {
		response := "59 Bad Request\r\n"
		conn.Write([]byte(response))
		return
	}

	// if path has suffix "/" or empty, then append "index.gmi" to it
	if path == "" || strings.HasSuffix(path, "/") {
		path += "index.gmi"
	}

	file, mime, err := getFile(path)

	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			response := "51 Not Found\r\n"
			conn.Write([]byte(response))
			return
		}
	}

	_, err = conn.Write([]byte(fmt.Sprintf("20 %s\r\n", mime)))
	if err != nil {
		log.Printf("Failed to send response: %v", err)
		return
	}

	err = sendFile(file, conn)
	if err != nil {
		log.Printf("Failed to send file: %v", err)
		return
	}
}

func parseRequestPath(request string) (string, error) {
	request = strings.TrimSpace(request)
	if !strings.HasPrefix(request, "gemini://") {
		return "", ErrBadRequest
	}

	request = strings.TrimPrefix(request, "gemini://")

	parts := strings.SplitN(request, "/", 2)
	path := parts[1]

	return path, nil
}
