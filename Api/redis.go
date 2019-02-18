package wheel

import "github.com/garyburd/redigo/redis"

func CheckUser(cli redis.Conn, name string) (bool, error) {
	exist, err := redis.Bool(cli.Do("EXISTS", "store:User:"+name))
	return exist, err
}

func CheckItem(cli redis.Conn, item string) (int64, error) {
	count, err := redis.Int64(cli.Do("GET", "store:item:"+item))
	return count, err
}

func SetItem(cli redis.Conn, item string, count int64) error {
	_, err := cli.Do("SET", "store:item:"+item, count)
	return err
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
