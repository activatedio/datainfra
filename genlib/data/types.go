package data

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/activatedio/datainfra/genlib"
	"github.com/dave/jennifer/jen"
)

// Entry is a descriptor for a data type
type Entry struct {
	Type       reflect.Type
	Operations *genlib.Set[Operation]
	// Implementations are implementation-specific parameters
	Implementations []any
}

func (d Entry) GetJenHelper() JenHelper {

	res := JenHelper{
		InterfaceName: fmt.Sprintf("%sRepository", d.Type.Name()),
		StructName:    d.Type.Name(),
	}

	res.StructType = jen.Qual(d.Type.PkgPath(), d.Type.Name())

	var keys []reflect.StructField

	for i := 0; i < d.Type.NumField(); i++ {
		f := d.Type.Field(i)
		dt := parseTag(f.Tag.Get("data"))
		if dt.isKey {
			keys = append(keys, f)
		}
	}

	switch {
	case len(keys) == 1:
		res.KeyType = jen.Qual(keys[0].Type.PkgPath(), keys[0].Type.Name())
	case len(keys) > 1:
		keyTypeName := fmt.Sprintf("%sKey", d.Type.Name())
		res.KeyType = jen.Id(keyTypeName)

		fs := &jen.Statement{}

		fs.Add(jen.Commentf("%s is the key for %s", keyTypeName, d.Type.Name()))

		for _, key := range keys {
			fs.Add(jen.Id(key.Name).Qual(key.Type.PkgPath(), key.Type.Name()))
		}

		res.keyStmt = jen.Type().Id(keyTypeName).Struct(*fs...)
	}

	return res
}

type dataTag struct {
	isKey bool
}

func parseTag(tag string) dataTag {

	dt := dataTag{}

	for _, v := range strings.Split(tag, ",") {
		switch {
		case v == "key":
			dt.isKey = true
		}
	}

	return dt
}

type JenHelper struct {
	InterfaceName string
	StructType    jen.Code
	StructName    string
	KeyType       jen.Code
	// If not nil, we use this statement to generate a key, usually for a composite type
	keyStmt *jen.Statement
}

type Operation struct {
	slug string
}

func (o Operation) String() string {
	return o.slug
}

var (
	OperationFindByKey = Operation{"findByKey"}
	OperationsCrud     = genlib.NewFrozenSet(OperationFindByKey)
)
