package main

import (
	"fmt"
	"github.com/dchest/captcha"
	"github.com/garyburd/redigo/redis"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	"gostore/util"
	"io"
	"net/http"
	"runtime"
	"time"
)

var (
	FastStore = "FASTSTORE"
	sess      = sessions.New(sessions.Config{Cookie: FastStore})
	cli       *redis.Pool
)

func main() {
	app := iris.New()

	cli = &redis.Pool{
		MaxIdle:     400,
		MaxActive:   500,
		IdleTimeout: 240 * time.Second,
		Wait:        true,
		Dial: func() (conn redis.Conn, e error) {
			cli, err := redis.Dial("tcp", "127.0.0.1:6379")
			redis.DialConnectTimeout(300 * time.Second)
			redis.DialReadTimeout(300 * time.Second)
			redis.DialWriteTimeout(300 * time.Second)
			if err != nil {
				//logging.Println(err)
				return nil, err
			}
			return cli, err
		},
	}

	//使用中间件添加header信息
	app.Use(func(ctx iris.Context) {
		ctx.Header("Server", "Server/1.0")
		ctx.Header("X-Powered-By", "SJJ")
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Next()
	})

	api := app.Party("/api")
	{
		api.Post("/setItem", func(ctx iris.Context) {
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
			item := ctx.PostValue("item")
			describe := ctx.PostValue("describe")
			count := ctx.PostValue("count")
			//logging.Println("setItem", item, describe, count)
			if item == "" || describe == "" || count == "" {
				WriteJson(ctx, 10001, "设置失败，商品信息有误！", nil)
				return
			}
			err = util.SetItem(cli, item, describe, count)
			if err != nil {
				WriteJson(ctx, 10001, "设置失败，数据库出错辣！", nil)
				return
			}
			WriteJson(ctx, 0, "OK", nil)
		})
	}

	//验证码
	capt := app.Party("/captcha")
	{
		//返回验证码图片 [GET /captcha/create]
		capt.Get("/create", func(ctx iris.Context) {
			id := captcha.NewLen(4)
			session := sess.Start(ctx)
			session.Set("ID", id)
			ctx.StreamWriter(func(w io.Writer) bool {
				err := captcha.WriteImage(w, id, 240, 80)
				if err != nil {
					//logging.Debug(err)
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
		reg.Get("/checkUser", func(ctx iris.Context) {

			name := ctx.FormValue("name")
			exist, err := util.CheckUser(cli, name)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			}
			if exist {
				WriteJson(ctx, 10001, "用户名已存在", nil)
				return
			} else {
				//logging.Debug(name)
				WriteJson(ctx, 0, "OK", nil)
				return
			}
		})
		//增加新用户 [POST /register/addUser] 参数 name=xxx&nick=xxx&pwd=xxx&capt=xxx
		reg.Post("/addUser", func(ctx iris.Context) {
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
			exist, err := util.CheckUser(cli, name)
			if exist {
				WriteJson(ctx, 10001, "用户名已存在", nil)
				return
			}
			err = util.AddUser(cli, name, nick, pwd)
			if err != nil {
				//logging.Debug(err)
				return
			} else {
				//logging.Debug("new user: ", name)
			}
			ctx.Redirect("/")
		})
	}

	//登陆
	login := app.Party("/login")
	{
		//登陆 [POST /login/do] 参数 name=xxx&pwd=xxx&capt=xxxx
		login.Post("/do", func(ctx iris.Context) {
			name := ctx.PostValue("name")
			pwd := ctx.PostValue("pwd")
			capt := ctx.PostValue("capt")
			if name == "admin" && pwd == "admin" {
				session := sess.Start(ctx)
				session.Set("NAME", "admin")
				session.Set("NICK", "admin")
				session.Set("ADMIN", true)
				session.Set("AUTH", true)
				ctx.Redirect("/main/manage")
			}
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
			pwd = util.MD5String(pwd)
			exist, err := util.CheckUser(cli, name)
			if err != nil {
				//logging.Println(err)
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			} else {
				if exist {
					password, err := util.GetPwd(cli, name)
					if err != nil {
						WriteJson(ctx, 10000, "查询失败", nil)
						return
					}
					if password == pwd {
						nick, err := util.GetNickName(cli, name)
						session.Set("NAME", name)
						session.Set("NICK", nick)
						session.Set("AUTH", true)
						if err != nil {
							//logging.Println("登陆失败")
						}
						ctx.Redirect("/")
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
		login.Get("/out", func(ctx iris.Context) {
			session := sess.Start(ctx)
			session.Set("AUTH", false)
			//WriteJson(ctx, 0, "OK", nil)
			ctx.Redirect("/")
		})
	}

	main := app.Party("/main")
	{

		//抢购 [GET /main/purchase?item=xxx]
		main.Get("/purchase", func(ctx iris.Context) {
			if !LoginAuth(ctx) {
				return
			}
			item := ctx.FormValue("item")
			name := sess.Start(ctx).GetString("NAME")
			err := util.Purchase(cli, name, item)
			if err != nil {
				WriteJson(ctx, 10004, "已经抢光了，下次再试试吧", nil)
				return
			}
			WriteJson(ctx, 0, "OK", nil)
		})
		main.Get("/account", func(ctx iris.Context) {
			if !LoginAuth(ctx) {
				return
			}
			nick := sess.Start(ctx).GetString("NICK")
			name := sess.Start(ctx).GetString("NAME")
			count, err := util.GetPurchaseCount(cli, name)
			ctx.ViewData("nick", nick)
			ctx.ViewData("name", name)
			ctx.ViewData("count", count)
			err = ctx.View("account.html")
			if err != nil {
				return
			}
		})
		main.Get("/list", func(ctx iris.Context) {
			//ctx.Header("Cache-Control", "no-store")
			if !LoginAuth(ctx) {
				return
			}
			list, err := util.GetItemList(cli)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			}
			WriteSliceJson(ctx, 0, "OK", "list", list)
		})
		main.Get("/order", func(ctx iris.Context) {
			ctx.Header("Cache-Control", "no-store")
			if !LoginAuth(ctx) {
				return
			}
			name := sess.Start(ctx).GetString("NAME")
			list, err := util.GetPurchaseList(cli, name)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			}
			WriteSliceJson(ctx, 0, "OK", "list", list)
		})
		main.Get("/manage", func(ctx iris.Context) {
			session := sess.Start(ctx)
			admin, err := session.GetBoolean("ADMIN")
			if err != nil {
				ctx.Redirect("/")
				return
			}
			if !admin {
				ctx.Redirect("/")
				return
			}
			_ = ctx.View("manage.html")
		})
	}

	//主页
	app.Get("/", func(ctx iris.Context) {
		auth, _ := sess.Start(ctx).GetBoolean("AUTH")
		if !auth {
			_ = ctx.View("login.html") // 已经注册到www文件夹了
			return
		}
		nick := sess.Start(ctx).GetString("NICK")
		ctx.ViewData("nick", nick)
		_ = ctx.View("index.html")
	})

	//使用StaticWeb中间件处理静态文件
	//app.StaticWeb("/", "www")
	//app.StaticContent("/", "www")
	app.HandleDir("/", "./www")
	app.RegisterView(iris.HTML("www", ".html"))

	//自定义错误页面
	app.OnAnyErrorCode(func(ctx iris.Context) {
		code := ctx.GetStatusCode()
		ctx.ViewData("code", code)
		ctx.ViewData("msg", http.StatusText(code))
		ctx.ViewData("server", "Server/1.0")
		_ = ctx.View("error.html")
	})

	runtime.GOMAXPROCS(runtime.NumCPU()) //打开多核之后在多核机器上可以提升网络吞吐量
	fmt.Println(runtime.NumCPU())

	config := util.GetConfig()
	port := config["Port"].(string)
	err := app.Run(iris.Addr(port))
	if err != nil {
		return
	}
}

func WriteSliceJson(context iris.Context, code int, message string, name string, sli interface{}) {
	data := make(map[string]interface{})
	data["code"] = code
	data["message"] = message
	if sli != nil {
		data[name] = sli
	}
	_, err := context.JSON(data)
	if err != nil {
		return
	}
}

func WriteJson(context iris.Context, code int, message string, cbk func(map[string]interface{})) {
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

func LoginAuth(ctx iris.Context) bool {
	auth, _ := sess.Start(ctx).GetBoolean("AUTH")
	if !auth {
		ctx.Redirect("/")
		return false
	}
	return true
}
