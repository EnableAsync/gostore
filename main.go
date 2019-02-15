package main

import (
	"github.com/dchest/captcha"
	"github.com/garyburd/redigo/redis"
	"github.com/kataras/iris"
	"github.com/kataras/iris/context"
	"github.com/kataras/iris/sessions"
	"io"
	"net/http"
	"store/Api"
	"strconv"
)

var (
	FastStore = "FASTSTORE"
	sess      = sessions.New(sessions.Config{Cookie: FastStore})
)

func main() {
	app := iris.New()
	logging := app.Logger()
	logging.SetLevel("debug")

	cli, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		logging.Debug(err)
	}
	defer cli.Close()

	//使用中间件添加header信息
	app.Use(func(ctx context.Context) {
		ctx.Header("Server", "Server/1.0")
		ctx.Header("X-Powered-By", "SJJ")
		ctx.Next()
	})

	api := app.Party("/api")
	{
		api.Get("/setItem", func(ctx context.Context) {
			item := ctx.FormValue("item")
			str := ctx.FormValue("count")
			count, err := strconv.ParseInt(str, 10, 64)
			session := sess.Start(ctx)
			admin, err := session.GetBoolean("ADMIN")
			if err != nil {
				ctx.StatusCode(iris.StatusForbidden)
				return
			}
			if !admin {
				ctx.StatusCode(iris.StatusForbidden)
				return
			}
			err = wheel.SetItem(cli, item, count)
			if err != nil {
				WriteJson(ctx, 10001, "设置失败", nil)
				return
			}
		})
	}

	//验证码
	capt := app.Party("/captcha")
	{
		//返回验证码图片 [GET /captcha/create]
		capt.Get("/create", func(ctx context.Context) {
			id := captcha.NewLen(4)
			session := sess.Start(ctx)
			session.Set("ID", id)
			ctx.StreamWriter(func(w io.Writer) bool {
				err = captcha.WriteImage(w, id, 240, 80)
				if err != nil {
					logging.Debug(err)
					return false
				}
				return false
			})
		})
	}

	//注册
	reg := app.Party("/register")
	{
		//检测用户名是否存在 [GET /register/checkUser?name=xxx]
		reg.Get("/checkUser", func(ctx context.Context) {

			name := ctx.FormValue("name")
			exist, err := wheel.CheckUser(cli, name)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			}
			if exist {
				WriteJson(ctx, 10001, "用户名已存在", nil)
				return
			} else {
				logging.Debug(name)
				WriteJson(ctx, 0, "OK", nil)
			}
		})
		//增加新用户 [POST /register/addUser] 参数 name=xxx&nick=xxx&pwd=xxx&capt=xxx
		reg.Post("/addUser", func(ctx context.Context) {
			name := ctx.PostValue("name")
			nick := ctx.PostValue("nick")
			pwd := ctx.PostValue("pwd")
			capt := ctx.PostValue("capt")
			if name == "" || pwd == "" || nick == "" || capt == "" {
				WriteJson(ctx, 10000, "缺少参数", nil)
				return
			}
			session := sess.Start(ctx)
			id := session.GetString("ID")
			if id == "" {
				WriteJson(ctx, 10002, "验证码错误", nil)
				return
			}
			if !captcha.VerifyString(id, capt) {
				WriteJson(ctx, 10002, "验证码错误", nil)
				return
			}
			exist, err := wheel.CheckUser(cli, name)
			if exist {
				WriteJson(ctx, 10001, "用户名已存在", nil)
				return
			}
			err = wheel.AddUser(cli, name, nick, pwd)
			if err != nil {
				logging.Debug(err)
				return
			} else {
				logging.Debug("new user: ", name)
			}
			WriteJson(ctx, 0, "OK", nil)
		})
	}

	//登陆
	login := app.Party("/login")
	{
		//登陆 [POST /login/do] 参数 name=xxx&pwd=xxx&capt=xxxx
		login.Post("/do", func(ctx context.Context) {
			name := ctx.PostValue("name")
			pwd := ctx.PostValue("pwd")
			capt := ctx.PostValue("capt")
			if name == "" || pwd == "" || capt == "" {
				WriteJson(ctx, 10000, "缺少参数", nil)
				return
			}
			session := sess.Start(ctx)
			id := session.GetString("ID")
			if id == "" {
				WriteJson(ctx, 10002, "验证码错误", nil)
				return
			}
			if !captcha.VerifyString(id, capt) {
				WriteJson(ctx, 10002, "验证码错误", nil)
				return
			}
			pwd = wheel.MD5String(pwd)
			exist, err := wheel.CheckUser(cli, name)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			} else {
				if exist {
					password, err := wheel.GetPwd(cli, name)
					if err != nil {
						WriteJson(ctx, 10000, "查询失败", nil)
						return
					}
					if password == pwd {
						session.Set("NAME", name)
						session.Set("AUTH", true)
						//WriteJson(ctx, 0, "OK", nil)
						ctx.Redirect("/", iris.StatusMovedPermanently)
					} else {
						WriteJson(ctx, 10003, "用户名或密码错误", nil)
						return
					}
				} else {
					WriteJson(ctx, 10003, "用户名或密码错误", nil)
					return
				}
			}
		})
		//退出登陆 [POST /login/out]
		login.Get("/out", func(ctx context.Context) {
			session := sess.Start(ctx)
			session.Set("AUTH", false)
			WriteJson(ctx, 0, "OK", nil)
		})
	}

	main := app.Party("/main")
	{

		//抢购 [GET /main/purchase?item=xxx]
		main.Get("/purchase", func(ctx context.Context) {
			session := sess.Start(ctx)
			auth, err := session.GetBoolean("AUTH")
			if err != nil {
				logging.Debug(err)
				ctx.StatusCode(iris.StatusForbidden)
				return
			}
			if !auth {
				ctx.StatusCode(iris.StatusForbidden)
				return
			}
			item := ctx.FormValue("item")
			count, err := wheel.CheckItem(cli, item)
			if count <= 0 {
				WriteJson(ctx, 10004, "已经抢光辣，下次再试试吧", nil)
				return
			} else {
				name := session.GetString("name")
				_, err = cli.Do("WATCH", "store:item:"+item)
				_, err = cli.Do("MULTI")
				_, err = cli.Do("SET", "store:item:"+item, count-1)
				_, err = cli.Do("RPUSH", "store:purchase:"+item, name)
				_, err := cli.Do("EXEC")
				if err != nil {
					WriteJson(ctx, 10005, "抢购失败，请重试", nil)
					return
				}
				WriteJson(ctx, 0, "OK", nil)
			}
		})
	}

	//主页
	app.Get("/", func(ctx context.Context) {
		auth, err := sess.Start(ctx).GetBoolean("AUTH")
		if err != nil {
			err = ctx.View("login.html") // 已经注册到views文件夹了
			return
		}
		if !auth {
			err = ctx.View("login.html") // 已经注册到views文件夹了
			return
		}
		_, err = ctx.WriteString("The cake is a lie!")
	})

	//使用StaticWeb中间件处理静态文件
	app.StaticWeb("/", ".")

	//自定义错误页面
	app.RegisterView(iris.HTML("./views", ".html"))
	app.OnAnyErrorCode(func(ctx context.Context) {
		code := ctx.GetStatusCode()
		ctx.ViewData("code", code)
		ctx.ViewData("msg", http.StatusText(code))
		ctx.ViewData("server", "Server/1.0")
		_ = ctx.View("error.html")
	})

	err = app.Run(iris.Addr(":8081"))
	if err != nil {
		logging.Debug(err)
		return
	}
}

func WriteJson(context context.Context, code int, message string, cbk func(map[string]interface{})) {
	data := make(map[string]interface{})
	data["code"] = code
	data["message"] = message
	if cbk != nil {
		cbk(data)
	}
	_, err := context.JSON(data)
	if err != nil {
		return
	}
}
