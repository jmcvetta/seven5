package seven5

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"seven5/util"
	"strings"
	"time"
)

//simulate const array
func DEFAULT_IMPORTS() []string {
	return []string{"fmt", "seven5", "os"}
}

// Bootstrap is responsible for buliding the current seven5 executable
// based on the groupie configuration.
type bootstrap struct {
	request *http.Request
	logger  util.SimpleLogger
}

//Bootstrap is invoked from the roadie to tell us that the user wants to 
//try to build and run their project.  Normally, this results in a new
//Seven5 excutabel.
func Bootstrap(writer http.ResponseWriter, request *http.Request,
	logger util.SimpleLogger) string {

	start := time.Now()

	b := &bootstrap{request, logger}
	config := b.configureSeven5("")
	if config != nil {
		result := b.takeSeven5Pill(config)
		delta := time.Since(start)
		logger.Info("Rebuilding seven5 took %s", delta.String())
		return result
	}

	return ""
}

//configureSeven5 checks for a goroupie config file and returns a config or
//nil in the error case. pass "" to use current working dir.
func (self *bootstrap) configureSeven5(dir string) groupieConfig {

	var groupieJson string
	var err error
	var result groupieConfig

	groupieJson, err = util.ReadIntoString(dir, GROUPIE_CONFIG_FILE)
		
	if err != nil {
		self.logger.Error("unable find or open the groupies config:%s", err)
		return nil
	}
	self.logger.DumpJson(util.DEBUG, "Groupie configuration", groupieJson)

	if result, err = getGroupies(groupieJson, self.logger); err != nil {
		self.logger.DumpJson(util.ERROR,"Groupie configuration", groupieJson)
		self.logger.Error("could not understand groupie.json! aborting!")
		return nil
	}

	return result
}

//takeSeven5 generates the pill in a temp directory and compiles it.  It returns
//the name of the new seven5 command or "" if it failed.
func (self *bootstrap) takeSeven5Pill(config groupieConfig) string {
	var cmd string
	var errText string
	var imports bytes.Buffer
	var setStatement bytes.Buffer
	var err error
	
	seen := util.NewBetterList()
	for _, i := range DEFAULT_IMPORTS() {
		seen.PushBack(i)
	}
	//gather all includes
	for _, v := range config {
		for _, i := range v.ImportsNeeded {
			if seen.Contains(i) {
				continue
			}
			seen.PushBack(i)
		}
	}
	for e := seen.Front(); e != nil; e = e.Next() {
		imports.WriteString(fmt.Sprintf("import \"%s\"\n", e.Value))
	}

	//walk all the configed groupies
	for k, v := range config {
		setStatement.WriteString(fmt.Sprintf(
			"\tseven5.Seven5app[seven5.%s]=&%s{}\n",
			strings.ToUpper(k), v.TypeName))
	}

	mainCode := fmt.Sprintf(seven5pill,
		imports.String(),
		setStatement.String())

	self.logger.DumpTerminal(util.DEBUG, "Main code for seven5 pill", mainCode)

	if cmd, errText, err = util.CompilePill(mainCode, self.logger); cmd == "" {
		self.logger.DumpTerminal(util.ERROR, "Bogus seven5 pill code", mainCode)
		if errText!="" {
			self.logger.DumpTerminal(util.ERROR, "Unable to compile the seven5pill!",
				errText)
		}
		if err!=nil {
			self.logger.Error("Internal seven5 error: %s",err)
		}
		return ""
	}
	path := strings.Split(cmd, string(filepath.Separator))
	self.logger.Info("Seven5 is now [tmpdir]/%s", path[len(path)-1])

	return cmd
}

//seven5pill is the text of the pill
const seven5pill = `
package main
%s

func main() {
%s
	if len(os.Args)<7 {
		os.Exit(1)
	}
	//double percent bceause run through sprintf twice
	fmt.Fprintf(os.Stdout,"%%s\n",seven5.RunCommand(os.Args[1], 
		os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]))
	os.Stdout.Sync()
}
`
