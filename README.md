# goemini, yet another gemini server

This project is a server implementation of the Gemini Protocol, a lightweight, secure, privacy-focused internet protocol alternative to HTTP.

## protocol overview

The Gemini Protocol is designed to be much simpler than HTTP, focusing on serving static content securely. It uses a single request-response model, and all connections are encrypted. Unlike HTTP, Gemini requests consist of a single line (a URL), and the response contains a small header followed by the body of the response, if any.

## requirements

- Go
- OpenSSL

## installation

### clone the repository

```bash
git clone https://github.com/remreren/goemini.git
```

Then open the project on your preferred editor.

### generate certificates

Generate your server's certificate and private key with OpenSSL:

```bash
openssl req -x509 -newkey rsa:4096 -keyout gemini.key -out gemini.cert -days 365 -nodes
```

### run the server

There are two ways to run the server: installing it as a Go binary or running it directly.

#### Option 1: Install
To install the server as a Go binary, use:

```bash
go install ./cmd/goemini
```

Then, you can run the server using the binary:

```bash
/path/to/binary --cert gemini.cert --key gemini.key --path ./content/
```

#### Option 2: Run Directly
To run the server directly without installing:

```bash
go run ./cmd/goemini --cert gemini.cert --key gemini.key --path ./content/
```

### access the server

You can now access the server using any Gemini client. I recommend using [Amfora](https://github.com/makew0rld/amfora)
