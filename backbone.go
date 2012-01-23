package seven5

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"seven5/store"
	"strings"
	"encoding/json"
	"text/template"
)

//models that have been found
var models = make(map[string][]string)
//services that have been found
var services = []Routable{}

//backboneModel is BackboneService that is part of the seven5 library.  This calls is used 
//to indicate that a given type is intended to be used on the client side.  Note that types
//that are to be sent to the client side will be marshalled/unmarshalled by the json library of
//Go and thus will obey structure tags such as json="-" (which prevents the field from arriving
//at the client).  The first parameter should be all lowercase.
func backboneModel(singularName string, ptrToStruct interface{}) {
	fields := []string{}

	v := reflect.ValueOf(ptrToStruct)
	if v.Kind() != reflect.Ptr {
		panic("backbone models must be a pointer to a struct")
	}
	s := v.Elem()
	if s.Kind() != reflect.Struct {
		panic("backbone models must be a pointer to a struct")
	}
	t := s.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Name
		tag := f.Tag.Get("json")
		if tag != "" {
			if tag == "-" {
				continue
			}
		}
		fields = append(fields, name)
	}
	models[strings.ToLower(singularName)] = fields
}

//BackboneServiceis called by the "glue" code between a user-level package (application) and the seven5
//library to indicate to indicate the service that can implement storage and validation for the
//particular name.  Note that the actual URL will be /api/plural and the plural is computed via the
//Pluralize() function.  The signular name must be english and should be lower case.  The last parameter
//is an example of the type to be manipulated that is analyzed for fields that can be shown to the
//client side code.
//
//Most calls to this function are autogenerated by tune and look like this for struct Foo in a user
//package named pkg (note capitalization):
//BackboneService("foo",pkg.NewFooSvc(),&pkg.Foo{})
func BackboneService(singularName string, svc Routable, example interface{}) {
	services=append(services,svc)
	backboneModel(singularName, example)
}

//getBackboneServices returns the set of registered backbone services. This should never be needed
//by user-level and it is called at startup via WebAppRun().
func backboneServices() []Routable {
	return services
}

//modelGuise is responsible for shipping backbone models to the client.  It takes in models (structures
//in go) and spits out Javascript that allow the client-side to have a model of the same structure
//with the same field names. The exception is the Id field in go, which is "id" (lowercase) in
//Javascript.
type modelGuise struct {
	//we need the implementation of the default HTTP machinery 
	*httpRunnerDefault
}

//Name returns "ModelGuise"
func (self *modelGuise) Name() string {
	return "ModelGuise" //used to generate the UniqueId so don"t change this
}

//Pattern returns "/api/seven5/models" because this guise is part of the seven5 api.  This is not a 
//rest API.
func (self *modelGuise) Pattern() string {
	return "/api/seven5/models"
}

//AppStarting is called by the infrastructure to tell the guise that the application is starting.
//Unused for now.
func (self *modelGuise) AppStarting(log *log.Logger, store store.T) error {
	return nil
}

//newModelGuise creates a new ModelGuise.. but only one should be needed in any program.  This is created
//by the infrastructure and user-level code should never need to call this.
func newModelGuise() *modelGuise {
	return &modelGuise{newHttpRunnerDefault()}
}

func (self *modelGuise) ProcessRequest(req *http.Request) *http.Response {
	resp := new(http.Response)
	buffer := NewBufferCloser()
	t := template.Must(template.New("js").Parse(modelTemplate))
	for model, fields := range models {
		data := make(map[string]interface{})
		data["modelName"] = model
		data["modelNamePlural"] = Pluralize(model)
		data["fields"] = fields
		if err := t.Execute(buffer, data); err != nil {
			fmt.Fprintf(os.Stderr, "error writing model:%s\n", err)
			resp.StatusCode = 500
			resp.Status= err.Error()
			return resp
		}
	}
	resp.ContentLength = int64(buffer.Len())
	resp.Body = buffer
	return resp
}

//Plural takes (should take) a noun in the singular and returns the plural of the noun. 
//Based on http://code.activestate.com/recipes/577781.   Only understands english and lower case
//input.
func Pluralize(singular string) string {
	if singular == "" {
		return ""
	}
	aberrant, ok := aberrant_plural_map[singular]
	if ok {
		return aberrant
	}

	if len(singular) < 4 {
		return singular + "s"
	}

	root := singular
	suffix:=""

	switch {
	case negSlice(-1, root) == "y" && isVowel(negSlice(-2, root))==false:
		root = root[0 : len(root)-1]
		suffix = "ies"
	case negSlice(-1, singular) == "s":
		switch {
		case isVowel(negSlice(-2, singular)):
			if singular[len(singular)-3:] == "ius" {
				root = singular[0 : len(singular)-2]
				suffix = "i"
			} else {
				root = singular[0 : len(singular)-1]
				suffix = "ses"
			}
		default:
			suffix = "es"
		}
	case singular[len(singular)-2:] == "ch", singular[len(singular)-2:] == "sh":
		suffix = "es"
	default:
		suffix = "s"
	}

	return root + suffix
}

//aberrant_plural_map shows english is a weird and wonderful language
var aberrant_plural_map = map[string]string{
	"appendix":   "appendices",
	"barracks":   "barracks",
	"cactus":     "cacti",
	"child":      "children",
	"criterion":  "criteria",
	"deer":       "deer",
	"echo":       "echoes",
	"elf":        "elves",
	"embargo":    "embargoes",
	"focus":      "foci",
	"fungus":     "fungi",
	"goose":      "geese",
	"hero":       "heroes",
	"hoof":       "hooves",
	"index":      "indices",
	"knife":      "knives",
	"leaf":       "leaves",
	"life":       "lives",
	"man":        "men",
	"mouse":      "mice",
	"nucleus":    "nuclei",
	"person":     "people",
	"phenomenon": "phenomena",
	"potato":     "potatoes",
	"self":       "selves",
	"syllabus":   "syllabi",
	"tomato":     "tomatoes",
	"torpedo":    "torpedoes",
	"veto":       "vetoes",
	"woman":      "women",
}

//vowels is the set of vowels
var vowels = []string{"a", "e", "i", "o", "u"}

//negslice can compute a negative slice ala python
func negSlice(n int, s string) string {
	if n >= 0 {
		panic("bad negative slice index!")
	}
	if -n > len(s) {
		panic("negative slice index is too big")
	}
	i := len(s) + n //subtraction
	return s[i : i+1]
}

//isVowel returns true if a string of 1 char is a vowel
func isVowel(s string) bool {
	if len(s) != 1 {
		panic("bad call to isVowel")
	}
	for _,v := range vowels {
		if s == v {
			return true
		}
	}
	return false
}

//PrivateString is a variant of string that differs only in that it never marshals itself
//into JSON.  It's a good choice for fields values in structures that you don't want to be exposed
//to the client side via models, like password hashes or email.  If you wish to not even
//indicate the presence of the field, this can be combined with the annotation json:"omitempty"
//(doesn't work now, see http://code.google.com/p/go/issues/detail?id=2761)
type PrivateString string

//MarshalJSON always returns the empty value because it's PRIVATE.
func (self PrivateString) MarshalJSON() ([]byte, error) {
	fmt.Printf("calling marshal... true value is '%s'\n",string(self))
	return json.Marshal("")
	//return []byte(x),nil
}

//UnMarshalJSON does a string unmarshal for itself.
func (self *PrivateString) UnmarshalJSON(b []byte) error {
	//storage:=make([]byte,len(b))
	//copy(storage,b)
	*self=PrivateString(string(b[1:len(b)-1]))
	return nil
}

//PrivateBool is a variant of bool that differs only in that it never marshals itself
//into JSON.  It's a good choice for fields in structures that you don't want to be exposed
//to the client side via models, like internal flags.  If you wish to not even
//indicate the presence of the field, this can be combined with the annotation json:"omitempty"
//(doesn't work now, see http://code.google.com/p/go/issues/detail?id=2761)
type PrivateBool bool

//MarshalJSON always returns the empty value because it's PRIVATE bool.
func (self *PrivateBool) MarshalJSON() ([]byte, error) {
	return []byte(`""`),nil
}

//UnMarshalJSON does a bool unmarshal for itself.
func (self *PrivateBool) UnmarshalJSON(b []byte) error {
	s:=string(b)
	s=strings.ToLower(s)
	switch (s) {
	case "true":
		*self=PrivateBool(true)
	case "false":
		*self=PrivateBool(false)
	default:
		panic("it's a bool but it has some value besides true and false!")
	}
	return nil
}

const modelTemplate = `
window.{{.modelName}} = Backbone.Model.extend({
	{{range .fields}} {{.}} : null,
	{{end}}
defaults: function(){
	this.urlRoot="/{{.modelNamePlural}}"
}
});`
