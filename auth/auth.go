package auth

import (
	"net/http"
	"regexp"
	"strings"
)

type AuthInfo struct {
	// Usernane or email
	Username string
	// Plaintext password or token
	Password string

	// repo component of URL
	// Usually: "username/repo_name"
	// But could also be: "some_repo.git"
	Repo string

	// Are we pushing or fetching ?
	Push  bool
	Fetch bool

	Request *http.Request
}

var (
	repoNameRegex = regexp.MustCompile("^/?(.*?)/(HEAD|git-upload-pack|git-receive-pack|info/refs|objects/.*)$")
)

func Authenticator(authf func(AuthInfo) (bool, error)) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authName, authPass, ok := req.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="git server"`)
				http.Error(w, "request header has no authorization header", http.StatusUnauthorized)
				return
			}

			// Build up info from request headers and URL
			info := AuthInfo{
				Username: authName,
				Password: authPass,
				Repo:     repoName(req.URL.Path),
				Push:     isPush(req),
				Fetch:    isFetch(req),

				Request: req,
			}

			// Call authentication function
			authenticated, err := authf(info)
			if err != nil {
				code := http.StatusInternalServerError
				msg := err.Error()
				if se, ok := err.(StatusError); ok {
					code = se.StatusCode()
				}
				http.Error(w, msg, code)
				return
			}

			// Deny access to repo
			if !authenticated {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Access granted
			handler.ServeHTTP(w, req)
		})
	}
}

func isFetch(req *http.Request) bool {
	return isService("upload-pack", req)
}

func isPush(req *http.Request) bool {
	return isService("receive-pack", req)
}

func isService(service string, req *http.Request) bool {
	return getServiceType(req) == service || strings.HasSuffix(req.URL.Path, service)
}

func repoName(urlPath string) string {
	matches := repoNameRegex.FindStringSubmatch(urlPath)
	if matches == nil {
		return ""
	}
	return matches[1]
}

func getServiceType(r *http.Request) string {
	service_type := r.FormValue("service")

	if s := strings.HasPrefix(service_type, "git-"); !s {
		return ""
	}

	return strings.Replace(service_type, "git-", "", 1)
}

// StatusCode is an interface allowing authenticators
// to pass down error's with an http error code
type StatusError interface {
	StatusCode() int
}
