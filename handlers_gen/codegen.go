package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/template"
)

//APIMethodProperties - apigen:api fields
type apiMethodProperties struct {
	URL    string `json:"url"`
	Auth   string `json:"auth"`
	Method string `json:"method"`
	Name   string
	Owner  string
	Params methodParamsStruct
}
type methodParamsStruct struct {
	StructName string
	fields     []validationProps
}

type validationProps struct {
	fieldname  string
	paramName  string
	fieldType  string
	required   bool
	min        int
	max        int
	enum       bool
	enumVals   []string
	defaultVal string
}

func (valp *validationProps) parseTag(tag string) {

	indx := strings.Index(tag, "apivalidator:")
	if indx == -1 {
		return
	}
	splt := tag[indx+len("apivalidator:")+1 : len(tag)-1]
	fmt.Println(splt)
	spltArr := strings.Split(splt, ",")
	fmt.Println(spltArr)
	for _, elem := range spltArr {
		spltElem := strings.Split(elem, "=")
		if len(spltElem) < 1 {
			continue
		}
		switch spltElem[0] {
		case "required":
			{
				valp.required = true
			}
		case "min":
			{
				i1, err := strconv.Atoi(spltElem[1])
				if err != nil {
					continue
				}
				valp.min = i1
			}
		case "max":
			{
				i1, err := strconv.Atoi(spltElem[1])
				if err != nil {
					continue
				}
				valp.max = i1
			}
		case "paramname":
			{
				valp.paramName = spltElem[1]
			}
		case "default":
			{
				valp.defaultVal = spltElem[1]
			}
		case "enum":
			{
				valp.enum = true
				valp.enumVals = strings.Split(spltElem[1], "|")
			}
		default:
			{
				continue
			}
		}
	}
}

type tpl struct {
	ApiName         string
	UsrCases        string
	MethodName      string
	ParamStructName string
	AuthCode        string
	MethodCheckCode string
	ParseGetCode    string
	ParsePostCode   string
}

var (
	intTpl = template.Must(template.New("httpHandlerTpl").Parse(`
	// {{.ApiName}}
	func (srv *{{.ApiName}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
{{.UsrCases}}
		default:
			resJSON, _ := json.Marshal(map[string]string{"error": "unknown method"})
			http.Error(w, string(resJSON), http.StatusNotFound)
			// 404
		}
	}
`))

	wrappetTpl = template.Must(template.New("wrapperHandlerTpl").Parse(`
	func (srv *{{.ApiName}}) wrapper{{.ApiName}}{{.MethodName}}(w http.ResponseWriter, r *http.Request) {
		// заполнение структуры params
		// валидирование параметров
		fmt.Println("")
		ctx := r.Context()
		var params *{{.ParamStructName}}

		{{.AuthCode}}

		{{.MethodCheckCode}}

		{{.ParseGetCode}}

		{{.ParsePostCode}}
	
		fmt.Printf("Request {{.MethodName}} for user'%s'\n", params.Login)
	
		res, err := srv.{{.MethodName}}(ctx, *params)
		if err != nil {
			switch err.(type) {
			case ApiError:
				err := err.(ApiError)
				resJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
				http.Error(w, string(resJSON), err.HTTPStatus)
			default:
				resJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
				http.Error(w, string(resJSON), 500)
			}
			return
		}
	
		resultJSON, _ := json.Marshal(map[string]interface{}{"response": res, "error": ""})
		fmt.Fprintf(w, string(resultJSON))
	
		return
		// прочие обработки
	}
	`))
)

func makeWrapperMethodName(methodName string, parentStruct string) string {
	return "wrapper" + parentStruct + methodName
}

func writeHTTPHandlers(w io.Writer, meths []apiMethodProperties, structName string) {
	cases := ""
	for _, sre := range meths {
		cases += "\t\tcase \"" + sre.URL + "\" {\n\t\t\t"
		cases += makeWrapperMethodName(sre.Name, structName) + "(w,r) \n\t\t}\n"
	}
	var tl tpl
	tl.ApiName = structName
	tl.UsrCases = cases
	intTpl.Execute(w, tl)
}

func writeWrapper(w io.Writer, meth apiMethodProperties) {
	//здесь генерируем конкретный враппер для текущего метода
	//0 проверяем авторизацию
	//1 определяем тип запроса
	//2 находим параметры метода и объявляем заглушку для хранения параметров
	//3 парсим гет
	//4 парсим пост
	//5 валидируем
	//6 выполняем метод
	//7 парсим ошибки
	//8 выводим результат
	var tl tpl
	tl.ApiName = meth.Owner
	tl.MethodName = meth.Name
	tl.ParamStructName = meth.Params.StructName

	wrappetTpl.Execute(w, tl)
}

func main() {

	apis := map[string][]apiMethodProperties{}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out) // empty line

	for _, f := range node.Decls {
		g, ok := f.(*ast.FuncDecl)
		if !ok {
			//fmt.Printf("SKIP %T is not *ast.FuncDecl\n", f)
			continue
		}
		if g.Doc == nil {
			continue
		}

		fmt.Println(g.Name.String() + " ")
		tag := g.Doc.List[len(g.Doc.List)-1].Text

		indx := strings.Index(tag, "apigen:api")
		if indx == -1 {
			continue
		}

		res := apiMethodProperties{}
		json.Unmarshal([]byte(tag[indx+len("apigen:api"):]), &res)
		res.Name = g.Name.Name
		fmt.Println(tag)
		fmt.Println("A" + res.Auth)
		fmt.Println("M" + res.Method)
		fmt.Println("U" + res.URL)

		switch t := g.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			switch str := t.X.(type) {
			case *ast.Ident:
				position := fset.Position(str.NamePos)
				fmt.Printf("	%s:%d %s\r\n", position.Filename, position.Line, str.Name)
				res.Owner = str.Name
			default:
				continue
			}
		default:
			continue
		}

		parastr := g.Type.Params.List[1].Type.(*ast.Ident)
		res.Params.StructName = parastr.String()
		parastrObj := parastr.Obj.Decl.(*ast.TypeSpec)
		paratype := parastrObj.Type.(*ast.StructType)
		for _, vall := range paratype.Fields.List {
			var props validationProps
			props.fieldType = vall.Type.(*ast.Ident).String()
			props.fieldname = vall.Names[0].String()
			props.parseTag(vall.Tag.Value[1 : len(vall.Tag.Value)-1])
			res.Params.fields = append(res.Params.fields, props)
		}
		fmt.Printf(parastrObj.Name.Name)
		//нужно бы еще заполнить опции валидации
		writeWrapper(out, res)

		apis[res.Owner] = append(apis[res.Owner], res)

	}

	for strct, valr := range apis {
		writeHTTPHandlers(out, valr, strct)
	}

}

//как делать сам враппер пока не совсем очевидно, однако хттп обертку можно уже вполне сгенерировать
//нужно посмотреть как сделаны шаблоны в примере и применить непосредственно
