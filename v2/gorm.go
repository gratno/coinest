package main

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open("sqlite3", "./gorm.db")
	if err != nil {
		panic(err)
	}
	db.LogMode(false)
}
