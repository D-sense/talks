package main

import (
	"context"
	"fmt"
	"go/types"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// TODO: read flag -v for verbose, deactivate log otherwise
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if len(os.Args) != 2 {
		fmt.Println("usage: obfuscate typeName")
		return
	}
	// start_main OMIT
	targetTypeName := os.Args[1]

	pkg := loadPkg(ctx, logger)

	logger.Debug().Msgf("looking into package %q for type %q", pkg.Name, targetTypeName)

	// start_searchType OMIT
	// Lookup the type to obfuscate
	obj := pkg.Types.Scope().Lookup(targetTypeName) // HL
	if obj == nil {                                 // Nothing found
		logger.Fatal().Msgf("could not find type %q in package %q", targetTypeName, pkg.Name)
	}
	// end_searchType OMIT

	file := createFile(targetTypeName, logger)

	generateCode(obj, pkg, targetTypeName, file, logger)

	// start_closeFile OMIT
	if err := file.Close(); err != nil {
		logger.Fatal().Err(err).Msgf("could not close file %s", file.Name())
	}
	// end_closeFile OMIT

	// format the file
	runGoimport(file, logger)
	// end_main OMIT
}

func generateCode(obj types.Object, pkg *packages.Package, targetTypeName string, w io.Writer, logger zerolog.Logger) {
	// We'll use the underlying type to choose what to print
	// start_generateCode OMIT
	isString := underlyingString(obj) // HLunderlyingString

	vars := tmplVars{}                               // HLtmplVars
	vars.PkgName = pkg.Name                          // HLtmplVars
	vars.TypeName = targetTypeName                   // HLtmplVars
	vars.ReplaceBy = "*"                             // HLtmplVars
	vars.ReceiverName = receiverName(targetTypeName) // HLtmplVars

	t, err := template.New("tmpl").Parse(chooseTmpl(isString)) // HLparseTmpl
	if err != nil {
		logger.Fatal().Err(err).Msgf("could not parse template")
	}

	err = t.Execute(w, vars) // HLexecuteTmpl
	if err != nil {
		logger.Fatal().Msgf("could not execute template for %s", targetTypeName)
	}
	// end_generateCode OMIT

	return
}

// start_runGoimport OMIT
func runGoimport(file *os.File, logger zerolog.Logger) {
	cmdGoImports := exec.Command("goimports", "-w", file.Name()) // HL
	if out, err := cmdGoImports.CombinedOutput(); err != nil {   // HL
		logger.Fatal().Err(err).Msgf("could not run goimports: %v: %v\n%s",
			strings.Join(cmdGoImports.Args, " "),
			err,
			out)
	}
}

// end_runGoimport OMIT

// start_createFile OMIT
func createFile(targetTypeName string, logger zerolog.Logger) *os.File {
	fileName := fmt.Sprintf(fileNameTmpl, strings.ToLower(targetTypeName))
	f, err := os.Create(fileName)
	if err != nil {
		logger.Fatal().Msgf("could not create file %s", fileName)
	}

	return f
}

// end_createFile OMIT

func receiverName(targetTypeName string) string {
	receiverName, _ := utf8.DecodeRuneInString(targetTypeName) // gets the first letter of the type
	return strings.ToLower(string(receiverName))
}

// start_underlyingType OMIT
func underlyingString(obj types.Object) bool {
	return obj.Type().Underlying().String() == "string" // HL
}

// end_underlyingType OMIT

func loadPkg(ctx context.Context, logger zerolog.Logger) *packages.Package {
	// start_loadPkg OMIT
	// start_loadConf OMIT
	loadCfg := &packages.Config{
		Mode: packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedName,
		Context: ctx,
	}
	// end_loadConf OMIT

	pkgs, err := packages.Load(loadCfg) // HL
	if err != nil {
		logger.Fatal().Err(err).Msgf("could not load packages")
	}
	if len(pkgs) != 1 {
		logger.Fatal().Err(err).Msgf(
			"expecting only one package, received %d: %v", len(pkgs), pkgs)
	}
	pkg := pkgs[0]
	// end_loadPkg OMIT

	return pkg
}

// start_chooseTmpl OMIT
func chooseTmpl(isString bool) string {
	if isString {
		return fmt.Sprintf(codeTmpl, tmplStr, tmplStr)
	}
	return fmt.Sprintf(codeTmpl, tmplOther, tmplOther)
}

// end_chooseTmpl OMIT

type tmplVars struct {
	PkgName      string
	ReceiverName string
	TypeName     string
	ReplaceBy    string
}

const (
	// start_fileName OMIT
	fileNameTmpl = "gen_%s_obfuscated.go"
	// end_fileName OMIT

	// start_tmpl OMIT
	codeTmpl = `
// Code generated by obfuscate. DO NOT EDIT.
package {{.PkgName}}

import (
	"fmt"
	"strings"
)

func ({{.ReceiverName}} {{.TypeName}}) String() string {
	return %s
}

func ({{.ReceiverName}} {{.TypeName}}) GoString() string {
	return %s
}
`
	// end_tmpl OMIT

	// start_tmplPrint OMIT
	tmplStr   = `fmt.Sprint(strings.Repeat("{{.ReplaceBy}}", len(string(s))))`
	tmplOther = `fmt.Sprint(strings.Repeat("{{.ReplaceBy}}", 10))`
	// end_tmplPrint OMIT

)
