package seven5tool
const lib_test_tmpl=`
package {{.base}}

//Test code goes in this file

import (
	_ "github.com/seven5/seven5"
	"testing"
)

//NOTE: you can create these resources anytime/anywhere, because they are stateless
var {{.base}}Resource = &{{.name}}Resource{}


//test your logic by calling methods on the resources directly and checking the result objects directly
//no network required
func TestSomething(T *testing.T) {
	empty:=make(map[string]string)
	
	//empty header and empty query param test...
	result, err:={{.base}}Resource.Index(empty,empty)
	if err!=nil {
		T.Logf("no error, result was %s",result)
	}
}

`
