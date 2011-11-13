package seven5

import (
	"fmt"
	sqlite3 "github.com/mattn/go-sqlite3"
	"os"
	"path/filepath"
	"strings"
)

/*
handler_test = Handler(	send_spec='tcp://127.0.0.1:10070',
                       	send_ident='34f9ceee-cd52-4b7f-b197-88bf2f0ec378',
                       	recv_spec='tcp://127.0.0.1:10071',
			recv_ident='') 


main = Server(
    uuid="f400bf85-4538-4f7a-8908-67e313d515c2",
    access_log="/logs/access.log",
    error_log="/logs/error.log",
    chroot="./",
    default_host="localhost",
    name="test",
    pid_file="/run/mongrel2.pid",
    port=6767,
    hosts = [
        Host(name="localhost", routes={
            '/tests/': Dir(base='tests/', index_file='index.html',default_ctype='text/plain')
	    '/handlertest': handler_test
        })
    ]
)

servers = [main]
*/

func LocateProject(projectName string) (string, string) {
	_ = new(sqlite3.SQLiteDriver)
	cwd, _ := os.Getwd()
	arch := os.Getenv("GOARCH")
	localos := os.Getenv("GOOS")

	//this resets the cwd to the top of your project, assuming it exists
	//by assuming you are running in eclipse 
	eclipseBinDir := fmt.Sprintf("%s_%s", localos, arch)
	d, f := filepath.Split(cwd)
	if eclipseBinDir != "-" && eclipseBinDir == f {
		d = filepath.Clean(d)
		projectDir, b := filepath.Split(d)
		if b == "bin" {
			//possibly eclipse, switch to project root and look for
			//the project area
			pkg := filepath.Join(projectDir, "src", "pkg")
			info, _ := os.Stat(pkg)
			if info != nil && info.IsDirectory() {
				//probably running in eclipse, check the path to proj
				projectDir := filepath.Join(pkg, projectName)
				fmt.Printf("trying project dir %s\n", projectDir)
				info, _ = os.Stat(projectDir)
				if info != nil && info.IsDirectory() {
					return projectDir, cwd //in eclipse
				}
			}
		}
	}
	//maybe you are in the project dir?
	foundProject :=true
	candidate := cwd
	parts := strings.Split(projectName, string(filepath.Separator))
	for i := len(parts) - 1; i >= 0; i-- {
		child := parts[i]

		parent, kid := filepath.Split(candidate)
		if kid != child {
			foundProject=false
		}
		candidate = filepath.Clean(parent)
	}
	//did we walk up, checking package structure?	
	if foundProject {
		return cwd,cwd
	}
	
	//try the root of the big tarball
	guess := filepath.Join(cwd,projectName)
	info, _ := os.Stat(guess)
	if info!=nil && info.IsDirectory() {
		return guess,cwd
	}
	
	
	return "",cwd
}

func VerifyProjectLayout(projectPath string) bool {

	return true
}

func clearTestDB() {
}

func discoverHandlerNames() {
}

func generateHandlerConfig() {
}

func generate() {
}

func createDBTablesForMongrel2() {
}

const TABLEDEFS_SQL = `
CREATE TABLE handler (id INTEGER PRIMARY KEY,
    send_spec TEXT, 
    send_ident TEXT,
    recv_spec TEXT,
    recv_ident TEXT,
   raw_payload INTEGER DEFAULT 0,
   protocol TEXT DEFAULT 'json');
CREATE TABLE host (id INTEGER PRIMARY KEY, 
    server_id INTEGER,
    maintenance BOOLEAN DEFAULT 0,
    name TEXT,
    matching TEXT);
CREATE TABLE log(id INTEGER PRIMARY KEY,
    who TEXT,
    what TEXT,
    location TEXT,
    happened_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    how TEXT,
    why TEXT);
CREATE TABLE mimetype (id INTEGER PRIMARY KEY, mimetype TEXT, extension TEXT);
CREATE TABLE proxy (id INTEGER PRIMARY KEY,
    addr TEXT,
    port INTEGER);
CREATE TABLE route (id INTEGER PRIMARY KEY,
    path TEXT,
    reversed BOOLEAN DEFAULT 0,
    host_id INTEGER,
    target_id INTEGER,
    target_type TEXT);
CREATE TABLE server (id INTEGER PRIMARY KEY,
    uuid TEXT,
    access_log TEXT,
    error_log TEXT,
    chroot TEXT DEFAULT '/var/www',
    pid_file TEXT,
    default_host TEXT,
    name TEXT DEFAULT '',
    bind_addr TEXT DEFAULT "0.0.0.0",
    port INTEGER,
    use_ssl INTEGER default 0);
CREATE TABLE setting (id INTEGER PRIMARY KEY, key TEXT, value TEXT);
CREATE TABLE statistic (id SERIAL, 
    other_type TEXT,
    other_id INTEGER,
    name text,
    sum REAL,
    sumsq REAL,
    n INTEGER,
    min REAL,
    max REAL,
    mean REAL,
    sd REAL,
    primary key (other_type, other_id, name));
`
