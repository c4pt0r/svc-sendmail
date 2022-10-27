package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"os"
	"strings"

	"github.com/c4pt0r/log"
	"github.com/gin-gonic/gin"
)

var auth smtp.Auth

var (
	addr     = flag.String("addr", "localhost:8080", "http service address")
	logLevel = flag.String("log-level", "info", "log level")
)

type Attachment struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

type SendReq struct {
	From        string       `json:"from"`
	To          []string     `json:"to"`
	CC          []string     `json:"cc"`
	BCC         []string     `json:"bcc"`
	Title       string       `json:"title"`
	Body        string       `json:"body"`
	Attachments []Attachment `json:"attachments"`
}

func (r SendReq) String() string {
	return fmt.Sprintf("From: %s | To: %s | Title: %s | Body: %s", r.From, r.To, r.Title, r.Body)
}

func sendMail(auth smtp.Auth, from string, to []string, cc []string, bcc []string, subject, body string, attachments []Attachment) error {
	buf := bytes.NewBuffer(nil)
	withAttachments := len(attachments) > 0
	buf.WriteString(fmt.Sprintf("Subject: %s\n", subject))
	buf.WriteString(fmt.Sprintf("To: %s\n", strings.Join(to, ",")))
	if len(cc) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\n", strings.Join(cc, ",")))
	}

	if len(bcc) > 0 {
		buf.WriteString(fmt.Sprintf("Bcc: %s\n", strings.Join(bcc, ",")))
	}

	buf.WriteString("MIME-Version: 1.0\n")
	writer := multipart.NewWriter(buf)
	boundary := writer.Boundary()
	if withAttachments {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\n", boundary))
		buf.WriteString(fmt.Sprintf("--%s\n", boundary))
	} else {
		buf.WriteString("Content-Type: text/plain; charset=utf-8\n")
	}
	buf.WriteString(body)
	if withAttachments {
		for _, v := range attachments {
			buf.WriteString(fmt.Sprintf("\n\n--%s\n", boundary))
			buf.WriteString(fmt.Sprintf("Content-Type: %s\n", http.DetectContentType(v.Content)))
			buf.WriteString("Content-Transfer-Encoding: base64\n")
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%s\n", v.Filename))

			b := make([]byte, base64.StdEncoding.EncodedLen(len(v.Content)))
			base64.StdEncoding.Encode(b, v.Content)
			buf.Write(b)
			buf.WriteString(fmt.Sprintf("\n--%s", boundary))
		}
		buf.WriteString("--")
	}
	addr := "smtp.gmail.com:587"

	if err := smtp.SendMail(addr, auth, from, to, buf.Bytes()); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	from := os.Getenv("GMAIL_FROM")
	pass := os.Getenv("GMAIL_PASSWORD")
	if from == "" || pass == "" {
		log.Fatal("GMAIL_FROM or GMAIL_PASSWORD is empty")
	}
	auth = smtp.PlainAuth("", from, pass, "smtp.gmail.com")

	r := gin.Default()
	r.POST("/send", func(c *gin.Context) {
		var req SendReq
		jsonData, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err = json.Unmarshal(jsonData, &req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Infof("send mail: %s", req)
		// TODO attachments
		// attachments := map[string][]byte{}
		err = sendMail(auth, req.From, req.To, req.CC, req.BCC, req.Title, req.Body, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.Run(*addr)
}
