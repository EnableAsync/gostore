package main

import (
	"github.com/garyburd/redigo/redis"
	"github.com/kataras/iris"
	"github.com/kataras/iris/context"
	"github.com/kataras/iris/sessions"
	"net/http"
	"store/Api"
)

var (
	FastStore = "FASTSTORE"
	sess      = sessions.New(sessions.Config{Cookie:FastStore})
)

func checkUser(cli redis.Conn, name string) (bool, error) {
	exist, err := redis.Bool(cli.Do("EXISTS", name))
	return exist, err
}

func main() {
	app := iris.New()
	logging := app.Logger()
	logging.SetLevel("debug")

	cli, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		logging.Debug(err)
	}
	defer cli.Close()

	app.Use(func(ctx context.Context) {
		ctx.Header("Server", "Server/1.0")
		ctx.Header("X-Powered-By", "SJJ")
		ctx.Next()
	})

	capt := app.Party("/captcha")
	{
		capt.Get("/create", func(ctx context.Context) {
			//id := captcha.NewLen(4)
		})
	}

	reg := app.Party("/register")
	{
		reg.Get("/checkUser", func(ctx context.Context) {
			json := make(map[string]interface{})
			json["code"] = 0
			json["msg"] = "OK"
			name := ctx.FormValue("name")
			exist, err := checkUser(cli, name)
			if err != nil {
				json["code"] = 10000
				json["msg"] = "查询失败"
				_, err = ctx.JSON(json)
				return
			}
			if exist {
				json["code"] = 10001
				json["msg"] = "用户名已存在"
				_, err = ctx.JSON(json)
				return
			} else {
				logging.Debug(name)
				_,err = ctx.JSON(json)
			}
		})
		reg.Post("/addUser", func(ctx context.Context) {
			json := make(map[string]interface{})
			json["code"] = 0
			json["msg"] = "OK"
			name := ctx.PostValue("name")
			pwd := ctx.PostValue("pwd")
			if name == "" || pwd == "" {
				json["code"] = 10000
				json["msg"] = "缺少参数"
				_, _ = ctx.JSON(json)
				return
			}
			exist, err := checkUser(cli, name)
			if exist {
				json["code"] = 10001
				json["msg"] = "用户名已存在"
				_, err = ctx.JSON(json)
				return
			}
			pwd = wheel.MD5String(pwd)
			_, err = cli.Do("SET", name, pwd)
			if err != nil {
				logging.Debug("cannot set data")
				return
			} else {
				logging.Debug("new user: ", name)
			}
			_, err = ctx.JSON(json)
			if err != nil {
				logging.Debug(err)
				return
			}
		})
	}

	login := app.Party("/login")
	{
		login.Post("/do", func(ctx context.Context) {
			json := make(map[string]interface{})
			json["code"] = 0
			json["msg"] = "OK"
			name := ctx.PostValue("name")
			pwd := ctx.PostValue("pwd")
			if name == "" || pwd == "" {
				json["code"] = 10000
				json["msg"] = "缺少参数"
				_, err = ctx.JSON(json)
				return
			}
			pwd = wheel.MD5String(pwd)
			exist, err := checkUser(cli, name)
			if err != nil {
				logging.Debug(err)
				return
			} else {
				if exist {
					password, err := redis.String(cli.Do("GET", name))
					if err != nil {
						logging.Debug(err)
						return
					}
					if password == pwd {
						session := sess.Start(ctx)
						session.Set("AUTH", true)
						_, err = ctx.JSON(json)
					} else {
						json["code"] = 10000
						json["msg"] = "用户名或密码不正确"
						_, err = ctx.JSON(json)
						return
					}
				} else {
					json["code"] = 10000
					json["msg"] = "用户名或密码不正确"
					_, err = ctx.JSON(json)
					return
				}
			}
		})
		login.Get("/out", func(ctx context.Context) {
			session := sess.Start(ctx)
			session.Set("AUTH", false)
		})
	}
	
	main := app.Party("/main")
	{
		main.Get("/", func(ctx context.Context) {
			auth, err := sess.Start(ctx).GetBoolean("AUTH")
			if err != nil {
				logging.Debug(err)
				ctx.StatusCode(iris.StatusForbidden)
				return
			}
			if !auth {
				ctx.StatusCode(iris.StatusForbidden)
				return
			}
			_, err = ctx.WriteString("The cake is a lie!")
		})
	}


	app.RegisterView(iris.HTML("./views", ".html"))
	app.OnAnyErrorCode(func(ctx context.Context) {
		code := ctx.GetStatusCode()
		ctx.ViewData("code", code)
		ctx.ViewData("msg", http.StatusText(code))
		ctx.ViewData("server", "Server/1.0")
		_ = ctx.View("error.html")
	})

	err = app.Run(iris.Addr(":8080"))
	if err != nil {
		logging.Debug(err)
		return
	}
}
