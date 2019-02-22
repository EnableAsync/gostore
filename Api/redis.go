package wheel

import (
	"github.com/garyburd/redigo/redis"
	"strconv"
)

/*
store:User为已注册用户 如store:User:SJJ则表示有一个注册用户，用户名为SJJ
store:User:SJJ
{
	name: "SJJ",
	nick: "SJJ",
	pwd: "xxx"
}

store:Item为商品列表 如store:User:notepad则表示有一件商品，商品名为notepad
store:User:notepad
{
	item: "notepad"
	describe: "xxx",
	count: 10
}

store:Purchase为商品抢购到的用户列表 如store:Purchase:notepad则表示抢到了notepad的用户
store:Purchase:notepad
[SJJ, test, Tom]

store:User:xxx:List为用户抢购到的商品列表 如store:User:SJJ:List则表示SJJ抢到的商品列表
store:User:SJJ:List
[notepad, iphone]

store:List为所有商品的list
store:List
[notepad, iphone]
*/

func CheckUser(cli redis.Conn, name string) (bool, error) {
	exist, err := redis.Bool(cli.Do("EXISTS", "store:User:"+name))
	return exist, err
}

func AddUser(cli redis.Conn, name string, nick string, pwd string) error {
	_, err := cli.Do("HMSET", "store:User:"+name, "nick", nick, "pwd", MD5String(pwd))
	return err
}

func GetNickName(cli redis.Conn, name string) (string, error) {
	nick, err := redis.String(cli.Do("HGET", "store:User:"+name, "nick"))
	return nick, err
}

func GetPwd(cli redis.Conn, name string) (string, error) {
	password, err := redis.String(cli.Do("HGET", "store:User:"+name, "pwd"))
	return password, err
}

func CheckItem(cli redis.Conn, item string) (int64, error) {
	str, err := redis.String(cli.Do("HGET", "store:Item:"+item, "count"))
	count, _ := strconv.ParseInt(str, 10, 64)
	return count, err
}

func SetItem(cli redis.Conn, item string, describe string, count string) error {
	_, err := cli.Do("HMSET", "store:Item:"+item, "item", item, "describe", describe, "count", count)
	_, err = cli.Do("RPUSH", "store:List", item)
	return err
}

func GetItem(cli redis.Conn, itemname string) (map[string]string, error) {
	item, err := redis.StringMap(cli.Do("HGETALL", "store:Item:"+itemname))
	return item, err
}

func GetListCount(cli redis.Conn) (int, error) {
	count, err := redis.Int(cli.Do("LLEN", "store:List"))
	return count, err
}

func GetItemList(cli redis.Conn) ([]map[string]string, error) {
	count, err := GetListCount(cli)
	list, err := redis.Strings(cli.Do("LRANGE", "store:List", 0, count))
	var items []map[string]string
	for _, item := range list {
		context, err := GetItem(cli, item)
		if err != nil {
			return nil, err
		}
		items = append(items, context)
	}
	return items, err
}

func Purchase(cli redis.Conn, name string, item string, count int64) error {
	//_, err := cli.Do("WATCH", "store:Item:" + item)
	str := strconv.FormatInt(count-1, 10)
	_, err := cli.Do("MULTI")
	_, err = cli.Do("HSET", "store:Item:"+item, "count", str)
	_, err = cli.Do("RPUSH", "store:Purchase:"+item, name)
	_, err = cli.Do("RPUSH", "store:User:"+name+":List", item)
	_, err = cli.Do("EXEC")
	return err
}

func GetPurchaseCount(cli redis.Conn, name string) (int, error) {
	count, err := redis.Int(cli.Do("LLEN", "store:User:"+name+":List"))
	return count, err
}

func GetPurchaseList(cli redis.Conn, name string) ([]string, error) {
	count, err := GetPurchaseCount(cli, name)
	list, err := redis.Strings(cli.Do("LRANGE", "store:User:"+name+":List", 0, count))
	return list, err
}
