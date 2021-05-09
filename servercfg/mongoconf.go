package servercfg

import (
        "github.com/gravitl/netmaker/config"
        "os"
)

func GetMongoUser() string {
	user := "mongoadmin"
	if os.Getenv("MONGO_ADMIN") != "" {
		user = os.Getenv("MONGO_ADMIN")
	} else if  config.Config.MongoConn.User != "" {
		user = config.Config.MongoConn.User
	}
	return user
}
func GetMongoPass() string {
        pass := "mongopass"
        if os.Getenv("MONGO_PASS") != "" {
                pass = os.Getenv("MONGO_PASS")
        } else if  config.Config.MongoConn.Pass != "" {
                pass = config.Config.MongoConn.Pass
        }
        return pass
}
func GetMongoHost() string {
        host := "127.0.0.1"
        if os.Getenv("MONGO_HOST") != "" {
                host = os.Getenv("MONGO_HOST")
        } else if  config.Config.MongoConn.Host != "" {
                host = config.Config.MongoConn.Host
        }
        return host
}
func GetMongoPort() string {
        port := "27017"
        if os.Getenv("MONGO_PORT") != "" {
                port = os.Getenv("MONGO_PORT")
        } else if  config.Config.MongoConn.Port != "" {
                port = config.Config.MongoConn.Port
        }
        return port
}
func GetMongoOpts() string {
        opts := "/?authSource=admin"
        if os.Getenv("MONGO_OPTS") != "" {
                opts = os.Getenv("MONGO_OPTS")
        } else if  config.Config.MongoConn.Opts != "" {
                opts = config.Config.MongoConn.Opts
        }
        return opts
}

