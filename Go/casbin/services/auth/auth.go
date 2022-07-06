package auth

import (
	"casbin/model"
	"casbin/pkgs"
	"errors"
	"github.com/casbin/casbin"
	"log"
	"net/http"
)

// Authorizer 认证中间件
func Authorizer(e *casbin.Enforcer, users model.Users) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// 从 session 中加载角色名称
			session := pkgs.SessionManager.Load(r)
			role, err := session.GetString("role")
			if err != nil {
				writeError(http.StatusInternalServerError, "内部错误", w, err)
				return
			}

			// 如果空 则为匿名用户
			if role == "" {
				role = "anonymous"
			}

			// if it's a member, check if user still exists
			if role == "member" {
				uid, err := session.GetInt("id")
				if err != nil {
					writeError(http.StatusInternalServerError, "内部错误", w, err)
					return
				}
				exists := users.Exists(uid)
				if !exists {
					writeError(http.StatusForbidden, "未授权页面", w, errors.New("用户不存在"))
					return
				}
			}

			// casbin enforce 判断是否具有路径权限
			res, err := e.EnforceSafe(role, r.URL.Path, r.Method)
			if err != nil {
				writeError(http.StatusInternalServerError, "内部错误", w, err)
				return
			}
			if res {
				next.ServeHTTP(w, r)
			} else {
				writeError(http.StatusForbidden, "未授权页面", w, errors.New("未授权"))
				return
			}
		}
		return http.HandlerFunc(fn)
	}
}

func writeError(status int, message string, w http.ResponseWriter, err error) {
	log.Print("Error: ", err.Error())
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}
