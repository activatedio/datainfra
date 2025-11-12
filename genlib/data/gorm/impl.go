package gorm

import "github.com/activatedio/datainfra/genlib/data"

type Implementation struct {
	// TableName allows overriding of the table name
	TableName       string
	RegistryBuilder data.RegistryBuilder
}
