package wheel

import (
	"errors"
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

func CheckUser(cli *redis.Pool, name string) (bool, error) {
	rc := cli.Get()
	defer rc.Close()
	exist, err := redis.Bool(rc.Do("EXISTS", "store:User:"+name))
	return exist, err
}

func AddUser(cli *redis.Pool, name string, nick string, pwd string) error {
	rc := cli.Get()
	defer rc.Close()
	_, err := rc.Do("HMSET", "store:User:"+name, "nick", nick, "pwd", MD5String(pwd))
	_, err = rc.Do("SAVE")
	return err
}

func GetNickName(cli *redis.Pool, name string) (string, error) {
	rc := cli.Get()
	defer rc.Close()
	nick, err := redis.String(rc.Do("HGET", "store:User:"+name, "nick"))
	return nick, err
}

func GetPwd(cli *redis.Pool, name string) (string, error) {
	rc := cli.Get()
	defer rc.Close()
	password, err := redis.String(rc.Do("HGET", "store:User:"+name, "pwd"))
	return password, err
}

func CheckItem(cli *redis.Pool, item string) (int64, error) {
	rc := cli.Get()
	defer rc.Close()
	str, err := redis.String(rc.Do("GET", "store:Item:"+item+":Count"))
	count, _ := strconv.ParseInt(str, 10, 64)
	return count, err
}

func ItemExist(cli *redis.Pool, item string) (bool, error) {
	rc := cli.Get()
	defer rc.Close()
	exist, err := redis.Bool(rc.Do("EXISTS", "store:Item:"+item))
	return exist, err
}

func SetItem(cli *redis.Pool, item string, describe string, count string) error {
	rc := cli.Get()
	defer rc.Close()
	exist, err := ItemExist(cli, item)
	_, err = rc.Do("HMSET", "store:Item:"+item, "item", item, "describe", describe, "count", count)
	remain, err := strconv.ParseInt(count, 10, 64)
	_, err = rc.Do("SET", "store:Item:"+item+":Count", remain)
	if !exist {
		_, err = rc.Do("RPUSH", "store:List", item)
	}
	_, err = rc.Do("SAVE")
	return err
}

func GetItem(cli *redis.Pool, itemname string) (map[string]string, error) {
	rc := cli.Get()
	defer rc.Close()
	item, err := redis.StringMap(rc.Do("HGETALL", "store:Item:"+itemname))
	return item, err
}

func GetListCount(cli *redis.Pool) (int, error) {
	rc := cli.Get()
	defer rc.Close()
	count, err := redis.Int(rc.Do("LLEN", "store:List"))
	return count, err
}

func GetItemList(cli *redis.Pool) ([]map[string]string, error) {
	rc := cli.Get()
	defer rc.Close()
	count, err := GetListCount(cli)
	list, err := redis.Strings(rc.Do("LRANGE", "store:List", 0, count))
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

func Purchase(cli *redis.Pool, name string, item string) error {
	rc := cli.Get()
	defer rc.Close()
	_, err := rc.Do("WATCH", "store:Item:"+item+":Count") //WATCH在前 防止超卖
	strremain, err := redis.String(rc.Do("GET", "store:Item:"+item+":Count"))
	remain, err := strconv.ParseInt(strremain, 10, 64)
	if remain > 0 {
		_, err = rc.Do("MULTI")
		_, err = rc.Do("SET", "store:Item:"+item+":Count", remain-1)
		_, err = rc.Do("HSET", "store:Item:"+item, "count", remain-1)
		_, err = rc.Do("RPUSH", "store:Purchase:"+item, name)
		_, err = rc.Do("RPUSH", "store:User:"+name+":List", item)
		_, err = rc.Do("SAVE")
		_, err = rc.Do("EXEC")
		return err
	}
	return errors.New("抢购太快啦，请重新试试吧")
}

func GetPurchaseCount(cli *redis.Pool, name string) (int, error) {
	rc := cli.Get()
	defer rc.Close()
	count, err := redis.Int(rc.Do("LLEN", "store:User:"+name+":List"))
	return count, err
}

func GetPurchaseList(cli *redis.Pool, name string) ([]string, error) {
	rc := cli.Get()
	defer rc.Close()
	count, err := GetPurchaseCount(cli, name)
	list, err := redis.Strings(rc.Do("LRANGE", "store:User:"+name+":List", 0, count))
	return list, err
}
