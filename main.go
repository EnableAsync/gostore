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
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Next()
	})

	api := app.Party("/api")
	{
		api.Get("/setItem", func(ctx context.Context) {
			item := ctx.FormValue("item")
			count := ctx.FormValue("count")
			describe := ctx.FormValue("describe")
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
			err = wheel.SetItem(cli, item, describe, count)
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
				return
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
			ctx.Redirect("/")
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
						nick, err := wheel.GetNickName(cli, name)
						session.Set("NAME", name)
						session.Set("NICK", nick)
						session.Set("AUTH", true)
						if err != nil {
							logging.Println("登陆失败")
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
		login.Get("/out", func(ctx context.Context) {
			session := sess.Start(ctx)
			session.Set("AUTH", false)
			//WriteJson(ctx, 0, "OK", nil)
			ctx.Redirect("/")
		})
	}

	main := app.Party("/main")
	{

		//抢购 [GET /main/purchase?item=xxx]
		main.Get("/purchase", func(ctx context.Context) {
			if !LoginAuth(ctx) {
				return
			}
			item := ctx.FormValue("item")
			count, err := wheel.CheckItem(cli, item)
			str := strconv.FormatInt(count, 10)
			logging.Println(item + ":" + str)
			if err != nil {
				logging.Println(err)
			}
			if count <= 0 {
				WriteJson(ctx, 10004, "已经抢光辣，下次再试试吧", nil)
				return
			} else {
				name := sess.Start(ctx).GetString("NAME")
				err = wheel.Purchase(cli, name, item, count)
				if err != nil {
					WriteJson(ctx, 10005, "抢购失败，请重试", nil)
					return
				}
				WriteJson(ctx, 0, "OK", nil)
			}
		})
		main.Get("/account", func(ctx context.Context) {
			if !LoginAuth(ctx) {
				return
			}
			nick := sess.Start(ctx).GetString("NICK")
			name := sess.Start(ctx).GetString("NAME")
			count, err := wheel.GetPurchaseCount(cli, name)
			ctx.ViewData("nick", nick)
			ctx.ViewData("name", name)
			ctx.ViewData("count", count)
			err = ctx.View("account.html")
			if err != nil {
				return
			}
		})
		main.Get("/list", func(ctx context.Context) {
			//ctx.Header("Cache-Control", "no-store")
			if !LoginAuth(ctx) {
				return
			}
			list, err := wheel.GetItemList(cli)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			}
			WriteSliceJson(ctx, 0, "OK", "list", list)
		})
		main.Get("/order", func(ctx context.Context) {
			ctx.Header("Cache-Control", "no-store")
			if !LoginAuth(ctx) {
				return
			}
			name := sess.Start(ctx).GetString("NAME")
			list, err := wheel.GetPurchaseList(cli, name)
			if err != nil {
				WriteJson(ctx, 10000, "查询失败", nil)
				return
			}
			WriteSliceJson(ctx, 0, "OK", "list", list)
		})
	}

	//主页
	app.Get("/", func(ctx context.Context) {
		auth, _ := sess.Start(ctx).GetBoolean("AUTH")
		if !auth {
			_ = ctx.View("login.html") // 已经注册到www文件夹了
			return
		}
		nick := sess.Start(ctx).GetString("NICK")
		ctx.ViewData("nick", nick)
		err = ctx.View("index.html")

		/*html, err := ioutil.ReadFile("./www/index.html")
		str := string(html[:])
		_, err = ctx.HTML(str)
		if err != nil {
			return
		}*/
	})

	//使用StaticWeb中间件处理静态文件
	app.StaticWeb("/", "www")

	//自定义错误页面
	app.RegisterView(iris.HTML("www", ".html"))
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

func WriteSliceJson(context context.Context, code int, message string, name string, sli interface{}) {
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

func LoginAuth(ctx context.Context) bool {
	auth, _ := sess.Start(ctx).GetBoolean("AUTH")
	if !auth {
		ctx.Redirect("/")
		return false
	}
	return true
}
