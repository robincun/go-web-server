package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func router(w http.ResponseWriter, r *http.Request) {
	log.Printf("Path: %s", r.URL.Path)
	log.Printf("Content-Type: %s", r.Header.Get("Content-Type"))
	log.Printf("Query Params: %v", r.URL.Query())
	if r.Method == "POST" || r.Method == "PUT" {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
		} else {
			log.Printf("Request Body: %s", string(bodyBytes))
		}
	}

	session := globalSessionManager.GetSession(r.RemoteAddr)

	for _, route := range *globalCustomRoutes {
		if route.Path == r.URL.Path {
			if route.IsAuthorizationNeeded && session.Authorized == false {
				HandleUnAuthorized(w, r)
				return
			}
			if route.IsExpirable && globalSessionManager.IsSessionExpired(*session) {
				HandleExpired(w, r)
				return
			}
			route.Handler(w, r, session)
			return
		}
	}

	if strings.Contains(r.URL.Path, "closed/") && session.Authorized == false {
		HandleUnAuthorized(w, r)
		return
	}

	if strings.Contains(r.URL.Path, "expirable/") && globalSessionManager.IsSessionExpired(*session) {
		HandleExpired(w, r)
		return
	}

	var targetSubfolder string

	ext := path.Ext(r.URL.Path)
	switch ext {
	case ".html":
		targetSubfolder = "pages"
	case ".css":
		targetSubfolder = "styles"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico":
		targetSubfolder = "images"
	case ".js":
		targetSubfolder = "scripts"
	default:
		targetSubfolder = "pages"
	}

	fmt.Println("Subfolder = " + targetSubfolder)
	filePath := filepath.Join("website", targetSubfolder, r.URL.Path)

	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {

		HandleNotFound(w, r, filePath)
		return

	} else if err != nil {
		HandleInternalServerError(w, r, filePath, err)
		return
	}

	http.ServeFile(w, r, filePath)
	session.UpdateLastConnectionTime()
	fmt.Print("Updatet Last Connection Time to", session.LastConnectionTime)
}

func serveCustomErrorPage(w http.ResponseWriter, statusCode int, pageFilename string) {
	w.WriteHeader(statusCode)

	errorPagePath := filepath.Join("website", "pages", "error", pageFilename)
	content, err := os.ReadFile(errorPagePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error %d: Could not load custom error page.", statusCode), statusCode)
		log.Printf("Failed to load custom error page %s: %v", errorPagePath, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func HandleNotFound(w http.ResponseWriter, r *http.Request, filePath string) {
	serveCustomErrorPage(w, http.StatusNotFound, "not-found-error.html")
	log.Printf("File not found: %s (requested URL: %s)", filePath, r.URL.Path)
}

func HandleUnAuthorized(w http.ResponseWriter, r *http.Request) {
	serveCustomErrorPage(w, http.StatusUnauthorized, "unauthorized-error.html")
}

func HandleExpired(w http.ResponseWriter, r *http.Request) {
	serveCustomErrorPage(w, http.StatusUnauthorized, "expired-session-error.html")
}

func HandleInternalServerError(w http.ResponseWriter, r *http.Request, filePath string, err error) {
	serveCustomErrorPage(w, http.StatusInternalServerError, "internal-server-error.html")
	log.Printf("Internal server error when accessing file %s: %v (requested URL: %s)", filePath, err, r.URL.Path)
}

var globalSessionManager *SessionManager
var globalCustomRoutes *[]CustomRoute

func StartServer(customRoutes []CustomRoute) error {
	mux := http.NewServeMux()

	globalSessionManager = NewSessionManager(30 * time.Second)
	globalCustomRoutes = &customRoutes

	mux.HandleFunc("/", router)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server started listening on port " + port)
	return http.ListenAndServe(port, mux)
}

type Session struct {
	LastConnectionTime time.Time
	Authorized         bool
}

func (session *Session) UpdateLastConnectionTime() {
	session.LastConnectionTime = time.Now()
}

func NewSession() *Session {
	session := &Session{}
	session.UpdateLastConnectionTime()
	return session
}

type SessionManager struct {
	sessionMap        map[string]*Session
	mutex             sync.RWMutex
	SessionExpiration time.Duration
}

func NewSessionManager(expiration time.Duration) *SessionManager {
	manager := &SessionManager{
		SessionExpiration: expiration,
		sessionMap:        make(map[string]*Session),
	}
	return manager
}

func (manager *SessionManager) GetSession(remoteAddr string) *Session {
	ipStr, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ipStr = remoteAddr
		fmt.Printf("Warning: Could not split host:port for %s, using full address as key.\n", remoteAddr)
	}

	manager.mutex.RLock()
	session, ok := manager.sessionMap[ipStr]
	manager.mutex.RUnlock()

	if ok {
		return session
	}

	newSession := NewSession()
	manager.sessionMap[ipStr] = newSession
	fmt.Println("new session created for IP: " + ipStr)
	return newSession
}

func (manager *SessionManager) IsSessionExpired(session Session) bool {
	if time.Now().Sub(session.LastConnectionTime) > manager.SessionExpiration {
		return true
	}
	return false
}

type CustomRoute struct {
	Path                  string
	IsAuthorizationNeeded bool
	IsExpirable           bool
	Handler               CustomHandler
}
type CustomHandler func(w http.ResponseWriter, r *http.Request, session *Session)
