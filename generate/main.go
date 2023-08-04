package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/quintans/gog/generator"
	"golang.org/x/exp/slices"
)

func main() {
	generator.Register(&Aspect{})

	if len(os.Args) < 2 {
		log.Println("please pass the dir to scan as an argument")
	}

	dir := os.Args[1]

	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatal(err)
	}

	generator.ScanDirAndSubDirs(absDir)
}

const (
	AspectMonitorTag = "@monitor"
)

type AspectOptions struct{}

type Aspect struct {
	generator.Scribler
	imports map[string]string
	fields  map[string]string
}

func (a Aspect) Name() string {
	return "aspect"
}

func (Aspect) Accepts() []generator.MapperType {
	return []generator.MapperType{
		generator.StructMapper,
	}
}

func (a Aspect) Imports(mapper generator.Mapper) map[string]string {
	return a.imports
}

func (a *Aspect) GenerateBody(mapper generator.Mapper) error {
	return a.WriteBody(mapper, AspectOptions{})
}

func (a *Aspect) WriteBody(mapper generator.Mapper, _ AspectOptions) error {
	sName := mapper.GetName() + "Aspect"

	for _, m := range mapper.GetMethods() {
		if !m.IsExported() {
			continue
		}

		a.BPrint("func (a *", sName, ") ", m.Signature(true), " {\n")

		methodName := "a.Next." + m.Name()

		tags := m.Tags.Filter(AspectMonitorTag)

		for k := len(tags) - 1; k >= 0; k-- {
			var (
				err  error
				body string
			)

			tag := m.Tags[k]
			switch tag.Name {
			case AspectMonitorTag:
				body, err = a.monitor(&m, mapper.GetName(), methodName)
				if err != nil {
					return err
				}
			default:
				continue
			}
			a.BPrintln("// ", tag.Name[1:], " aspect")
			methodName = fmt.Sprintf("fn%d", k)
			a.BPrint(methodName, " := ", body, "\n")
		}

		call := fmt.Sprint("(", m.Parameters(true), ")")
		if len(tags) > 0 {
			a.BPrintln("return fn0", call)
		} else {
			a.BPrintln("return ", methodName, call)
		}
		a.BPrint("}\n\n")
	}

	a.HPrintf("type %sAspect struct {\n", mapper.GetName())
	for _, v := range a.getFieldsOrdered() {
		a.HPrintln(v)
	}
	a.HPrintf("Next %s\n", mapper.GetName())
	a.HPrintf("}\n\n")

	return nil
}

func (a *Aspect) monitor(m *generator.Method, structName, methodName string) (string, error) {
	sign := m.Signature(false)
	s := generator.Scribler{}
	s.BPrintf("func%s{\n", sign)

	var errVar string
	rets := make([]string, 0, len(m.Results))

	for k, a := range m.Results {
		var v string
		if a.IsError() {
			v = "err"
			errVar = v
		} else {
			v = fmt.Sprintf("r%d", k)
		}
		rets = append(rets, v)

	}

	if errVar == "" {
		return "", fmt.Errorf("method %s must return an error type to use the 'monitor' aspect", m.Name())
	}

	s.BPrintln(strings.Join(rets, ","), " := ", methodName, "(", m.Parameters(true), ")")

	var sb generator.Scribler
	sb.BPrintln("map[string]any {")
	for _, v := range m.Args {
		if v.IsContext() {
			continue
		}
		sb.BPrintf("\"%s\": %s,\n", v.Name, v.Name)
	}
	sb.BPrint("}")
	s.BPrintf(`if %s != nil {
		a.Logger.WithError(err).
		WithTags(log.Tags{
			"method": "%s.%s",
			"arguments": utils.LazyStr(func() string {
				b, err := json.MarshalIndent(%s, "", "    ")
				if err != nil {
					return err.Error()
				}
				return string(b)
			}),
		}).
		Error("calling method")
	}
	`, errVar, structName, m.Name(), sb.String())

	s.BPrintln("return ", strings.Join(rets, ","))
	s.BPrintln("}")

	if a.imports == nil {
		a.imports = map[string]string{}
	}

	a.addField("Logger", "log.Logger", "github.com/quintans/eventsourcing/log")
	a.addImport("github.com/quintans/es-cqrs-bank-transfer/shared/utils")

	return s.String(), nil
}

func (a *Aspect) addField(name, kind, imp string) {
	a.addImport(imp)

	if a.fields == nil {
		a.fields = map[string]string{}
	}
	a.fields[name] = kind
}

func (a *Aspect) addImport(imp string) {
	if a.imports == nil {
		a.imports = map[string]string{}
	}
	a.imports[`"`+imp+`"`] = ""
}

func (a *Aspect) getFieldsOrdered() []string {
	fields := make([]string, 0, len(a.fields))
	for k, v := range a.fields {
		fields = append(fields, k+" "+v)
	}
	slices.Sort(fields)
	return fields
}
