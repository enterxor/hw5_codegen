package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
)

//APIMethodProperties - apigen:api fields
type apiMethodProperties struct {
	URL    string `json:"url"`
	Auth   bool   `json:"auth"`
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
	min_flag   bool
	max        int
	max_flag   bool
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
				valp.min_flag = true
				i1, err := strconv.Atoi(spltElem[1])
				if err != nil {
					continue
				}
				valp.min = i1
			}
		case "max":
			{
				valp.max_flag = true
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
	ValidationCode  string
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
		var params {{.ParamStructName}}

		{{.AuthCode}}

		{{.MethodCheckCode}}

		{{.ParseGetCode}}

		{{.ParsePostCode}}

		{{.ValidationCode}}
	
		fmt.Printf("Request {{.MethodName}} \n")
	
		res, err := srv.{{.MethodName}}(ctx, params)
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
		cases += "\t\tcase \"" + sre.URL + "\" :\n{\n\t\t\t"
		cases += "srv." + makeWrapperMethodName(sre.Name, structName) + "(w,r) \n\t\t}\n"
	}
	var tl tpl
	tl.ApiName = structName
	tl.UsrCases = cases
	intTpl.Execute(w, tl)
}

func errortojson(msg string, status int) string {
	res := ""
	res += "    resJSON, _ := json.Marshal(map[string]string{\"error\": \"" + msg + "\"})\n"
	res += "    http.Error(w, string(resJSON), " + strconv.Itoa(status) + ")\n"
	return res
}
func generateParamsForGET(meth *apiMethodProperties) string {
	res := "if r.Method == http.MethodGet {\n"
	for _, j := range meth.Params.fields {
		if len(j.paramName) < 1 {
			j.paramName = strings.ToLower(j.fieldname)
		}
		res += "" + j.fieldname + ",ok := r.URL.Query()[\"" + j.paramName + "\"]\n"
		if j.required {
			res += "if !ok || len(" + j.fieldname + "[0]) < 1 {\n"
			res += errortojson(j.paramName+" must me not empty", http.StatusBadRequest)
			res += "    return\n"
			res += "}\n"
		}
		if j.fieldType == "int" {
			res += "i1, err := strconv.Atoi(" + j.paramName + ")\n"
			res += "if err != nil {\n"
			res += errortojson(j.paramName+" must be int", http.StatusBadRequest)
			res += "}\n"
			res += "params." + j.fieldname + " = i1\n"
		} else {
			res += "params." + j.fieldname + " = " + j.fieldname + "[0]\n"
		}
	}

	res += "}\n"
	return res
}
func generateParamsForPOST(meth *apiMethodProperties) string {
	res := "if r.Method == http.MethodPost {\n"
	res += "r.ParseForm()\n"

	for _, j := range meth.Params.fields {
		if len(j.paramName) < 1 {
			j.paramName = strings.ToLower(j.fieldname)
		}

		res += j.fieldname + " := r.FormValue(\"" + j.paramName + "\")\n"
		res += "if len(" + j.fieldname + ") < 1 {\n"
		if j.required {
			res += errortojson(j.paramName+" must me not empty", http.StatusBadRequest)
			res += "    return\n"
		}
		res += "}else{\n"
		if j.fieldType == "int" {
			res += "i1, err := strconv.Atoi(" + j.fieldname + ")\n"
			res += "if err != nil {\n"
			res += errortojson(j.paramName+" must be int", http.StatusBadRequest)
			res += "    return\n"
			res += "}\n"
			res += "params." + j.fieldname + " = i1\n"
		} else {
			res += "params." + j.fieldname + " = " + j.fieldname + "\n"
		}
		res += "}\n"
	}

	res += "}\n"
	return res
}

func generateValidationCode(meth *apiMethodProperties) string {
	res := ""
	for _, j := range meth.Params.fields {
		if len(j.paramName) < 1 {
			j.paramName = strings.ToLower(j.fieldname)
		}
		if j.enum {
			res += "var " + j.fieldname + "Enum []string\n"
			for _, k := range j.enumVals {
				res += j.fieldname + "Enum = append(" + j.fieldname + "Enum," + "\"" + k + "\")\n"
				//" + strconv.Itoa(len(j.enumVals)) + "
				//res += j.fieldname + "Enum[" + strconv.Itoa(idx) + "] = \"" + k + "\"\n"
			}
			res += "if !contains(" + j.fieldname + "Enum, params." + j.fieldname + ") { \n"
			res += errortojson(j.paramName+" must be one of [\"+strings.Join("+j.fieldname+"Enum ,\", \" )+\"]", http.StatusBadRequest)
			res += "    return\n"
			res += "}\n"
		}
		if j.fieldType == "int" {
			if j.min_flag {
				res += "if params." + j.fieldname + " < " + strconv.Itoa(j.min) + " {\n"
				res += errortojson(j.paramName+" must be >= "+strconv.Itoa(j.min), http.StatusBadRequest)
				res += "    return\n"
				res += "}\n"
			}
			if j.max_flag {
				res += "if params." + j.fieldname + " > " + strconv.Itoa(j.max) + " {\n"
				res += errortojson(j.paramName+" must be <= "+strconv.Itoa(j.max), http.StatusBadRequest)
				res += "    return\n"
				res += "}\n"
			}
		} else {
			if j.min_flag {
				res += "if len(params." + j.fieldname + ") < " + strconv.Itoa(j.min) + " {\n"
				res += errortojson(j.paramName+" len must be >= "+strconv.Itoa(j.min), http.StatusBadRequest)
				res += "    return\n"
				res += "}\n"
			}
			if j.max_flag {
				res += "if len(params." + j.fieldname + ") > " + strconv.Itoa(j.max) + " {\n"
				res += errortojson(j.paramName+" len must be <= "+strconv.Itoa(j.max), http.StatusBadRequest)
				res += "    return\n"
				res += "}\n"
			}
		}

	}
	return res
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

	if meth.Auth {
		tl.AuthCode = "cookie:= r.Header.Get(\"X-Auth\")\n"
		tl.AuthCode += "if cookie != \"100500\" { \n"
		tl.AuthCode += "    resJSON, _ := json.Marshal(map[string]string{\"error\": \"unauthorized\"})\n"
		tl.AuthCode += "    http.Error(w, string(resJSON), 403)\n"
		tl.AuthCode += "    return\n"
		tl.AuthCode += "}\n"
	}

	tl.ParsePostCode = generateParamsForPOST(&meth)
	if meth.Method == "POST" {
		tl.MethodCheckCode += "if r.Method == http.MethodGet { \n"
		tl.MethodCheckCode += "    resJSON, _ := json.Marshal(map[string]string{\"error\": \"bad method\"})\n"
		tl.MethodCheckCode += "    http.Error(w, string(resJSON), http.StatusNotAcceptable)\n"
		tl.MethodCheckCode += "    return\n"
		tl.MethodCheckCode += "}\n"
	} else {
		tl.ParseGetCode = generateParamsForGET(&meth)
	}

	tl.ValidationCode = generateValidationCode(&meth)

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
	fmt.Fprintln(out, `import "strconv"`)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `	func contains(slice []string, item string) bool {
		set := make(map[string]struct{}, len(slice))
		for _, s := range slice {
			set[s] = struct{}{}
		}
	
		_, ok := set[item] 
		return ok
	}`)

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
		fmt.Println("A" + strconv.FormatBool(res.Auth))
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
