package shell

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samlm0/als/v2/als/client"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return strings.EqualFold(u.Host, r.Host)
	},
}

func HandleNewShell(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)
	handleNewConnection(conn, clientSession, c)
}

func handleNewConnection(conn *websocket.Conn, session *client.ClientSession, ginC *gin.Context) {
	ctx, cancel := context.WithCancel(session.Context())
	defer cancel()

	ex, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.CommandContext(ctx, ex, "--shell") // #nosec G204 command is fixed to current binary
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return
	}
	defer ptmx.Close()

	// context aware
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// cmd -> websocket
	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				break
			}
			if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
				break
			}
		}
	}()

	// websocket -> cmd
	go func() {
		defer cancel()
		for {
			_, buf, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if len(buf) < 1 {
				continue
			}
			index := string(buf[:1])
			switch index {
			case "1":
				// normal input
				if _, err := ptmx.Write(buf[1:]); err != nil {
					return
				}
			case "2":
				// win resize
				args := strings.Split(string(buf[1:]), ";")
				if len(args) < 2 {
					continue
				}
				h, errH := strconv.Atoi(args[0])
				w, errW := strconv.Atoi(args[1])
				if errH != nil || errW != nil {
					continue
				}
				if h <= 0 || h > int(^uint16(0)) || w <= 0 || w > int(^uint16(0)) {
					continue
				}
				if err := pty.Setsize(ptmx, &pty.Winsize{
					Rows: uint16(h),
					Cols: uint16(w),
				}); err != nil {
					return
				}
			}
		}
	}()
	if err := cmd.Wait(); err != nil && !errors.Is(err, syscall.ECHILD) {
		fmt.Println("shell command exited with error:", err)
	}
}
