package db

import (
	"log"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"golang.org/x/oauth2"
)

//Oauth2 describes the Oauth2 information needed for IMAP
type Oauth2 struct {
	ID    int64
	User  string
	Token *oauth2.Token
}

//Execute will connect to the DB
//will then execute the function fn passing this conencted db object
//and then close the connection
func Execute(fn func(db *pg.DB) error) error {
	db := pg.Connect(&pg.Options{
		User:     "admin",
		Password: "secret",
		Database: "postgres",
	})
	defer db.Close()

	err := fn(db)
	if err != nil {
		panic(err)
	}

	return nil
}

//InitilizeSchema initializes the schema for the database
func InitilizeSchema() error {
	log.Println("Initializing DB...")
	return Execute(func(db *pg.DB) error {
		models := []interface{}{
			(*Oauth2)(nil),
		}

		for _, model := range models {
			log.Println("Creating schema")
			err := db.Model(model).CreateTable(&orm.CreateTableOptions{
				IfNotExists: true,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

//SaveToken saves the token in DB
func SaveToken(user string, token *oauth2.Token) {
	Execute(func(db *pg.DB) error {
		oauth := new(Oauth2)
		err := db.Model(oauth).Where("oauth2.user = ?", user).Select()

		if err == pg.ErrNoRows {
			oauth.User = user
			oauth.Token = token
			_, err = db.Model(oauth).Insert()
		} else {
			oauth.Token = token
			_, err = db.Model(oauth).WherePK().Update()
		}
		return err
	})
}

//GetToken tries to retrieve a token from DB
func GetToken(user string) *oauth2.Token {
	oauth := new(Oauth2)
	Execute(func(db *pg.DB) error {
		log.Println("Trying to get the token from DB for user", user)
		err := db.Model(oauth).Where("oauth2.user = ?", user).Select()
		if err == pg.ErrNoRows {
			log.Println("No tokens found")
			err = nil
		}
		return err
	})
	return oauth.Token
}
