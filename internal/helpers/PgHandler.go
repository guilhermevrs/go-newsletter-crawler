package helpers

import (
	"log"

	"newsletter.crawler/internal/models"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"golang.org/x/oauth2"
)

type PgHandler struct{}

// NewPgHandler creates a new instance of the handler
func NewPgHandler() *PgHandler {
	return &PgHandler{}
}

//Execute will connect to the DB
//will then execute the function fn passing this conencted db object
//and then close the connection
func (pgh *PgHandler) Execute(fn func(db *pg.DB) error) error {
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
func (pgh *PgHandler) InitilizeSchema() error {
	log.Println("Initializing DB...")
	return pgh.Execute(func(db *pg.DB) error {
		models := []interface{}{
			(*models.Oauth2)(nil),
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
func (pgh *PgHandler) SaveToken(user string, token *oauth2.Token) error {
	return pgh.Execute(func(db *pg.DB) error {
		oauth := new(models.Oauth2)
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
func (pgh *PgHandler) GetToken(user string) (*oauth2.Token, error) {
	oauth := new(models.Oauth2)
	err := pgh.Execute(func(db *pg.DB) error {
		log.Println("Trying to get the token from DB for user", user)
		err := db.Model(oauth).Where("oauth2.user = ?", user).Select()
		if err == pg.ErrNoRows {
			log.Println("No tokens found")
			err = nil
		}
		return err
	})
	return oauth.Token, err
}
