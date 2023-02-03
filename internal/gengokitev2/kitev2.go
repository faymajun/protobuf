package gengokitev2

import (
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	ss     = protogen.GoImportPath("git.dhgames.cn/svr_comm/kite/v2/server/service")
	proto  = protogen.GoImportPath("google.golang.org/protobuf/proto")
	kite   = protogen.GoImportPath("git.dhgames.cn/svr_comm/kite/v2/kite")
	errors = protogen.GoImportPath("errors")
)

// GenerateFile generates a _kite.pb.go file containing gRPC service definitions.
func GenerateFile(gen *protogen.Plugin, file *protogen.File) *protogen.GeneratedFile {
	if len(file.Services) == 0 {
		return nil
	}
	filename := file.GeneratedFilenamePrefix + "_kite.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	g.P("// Code generated by protoc-gen-go-kite. DO NOT EDIT.")
	g.P()
	g.P("package ", file.GoPackageName)
	g.P()
	GenerateFileContent(gen, file, g)
	return g
}

// GenerateFileContent generates the gRPC service definitions, excluding the package statement.
func GenerateFileContent(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	if len(file.Services) == 0 {
		return
	}

	for _, service := range file.Services {
		genService(gen, file, g, service)
	}
}

func genService(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service) {
	clientName := export(service.GoName) + "Client"

	g.P("// ", clientName, " is the client API for ", service.GoName, " service.")
	g.P("//")
	g.P("// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/kite#ClientConn.NewStream.")

	// 全局
	g.P("var ", export(service.GoName), " = &", unExport(service.GoName), "{}")
	g.P()

	g.P("type ", unExport(service.GoName), " struct {")
	g.P("}")
	g.P()

	// NewClient factory.
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P(deprecationComment)
	}

	var methodIndex, streamIndex int
	// Client method implementations.
	for _, method := range service.Methods {
		if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
			// Unary RPC method
			genClientMethod(gen, file, g, method, methodIndex)
			methodIndex++
		} else {
			// Streaming RPC method
			genClientMethod(gen, file, g, method, streamIndex)
			streamIndex++
		}
	}

	// Server interface.
	serverType := export(service.GoName) + "Server"
	g.P("// ", serverType, " is the server API for ", service.GoName, " service.")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(deprecationComment)
	}
	g.Annotate(serverType, service.Location)
	g.P("type ", serverType, " interface {")
	for _, method := range service.Methods {
		g.Annotate(serverType+"."+method.GoName, method.Location)
		if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
			g.P(deprecationComment)
		}
		g.P(method.Comments.Leading,
			serverSignature(g, method))
	}
	g.P("}")
	g.P()

	// Server registration.
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P(deprecationComment)
	}

	g.P("type ", export(service.GoName), "Service struct {")
	g.P("handle ", export(service.GoName), "Server")
	g.P("}")
	g.P()

	fileName := path.Base(file.GeneratedFilenamePrefix)
	g.P("func Reg", export(service.GoName), "Server(handle ", serverType, ") {")
	g.P(g.QualifiedGoIdent(ss.Ident("Dispatch.Add")), `("`, fileName, `", "`, export(service.GoName), `", &`, export(service.GoName), "Service{handle: handle})")
	g.P("}")
	g.P()

	// Server handler implementations.

	// generate DO
	g.P("func (s *", export(service.GoName), "Service) Do(function string, reqPBData []byte, sender *kite.Destination) (resPBData []byte, err error) {")
	g.P("switch function {")
	for _, method := range service.Methods {
		g.P(`case "`, method.GoName, `":`)
		g.P("return s.", method.GoName, `(reqPBData, sender)`)
	}
	g.P("default:")
	g.P(`err = `, g.QualifiedGoIdent(errors.Ident(`New("function is not found")`)))
	g.P("}")
	g.P("return")
	g.P("}")
	g.P()

	// generate function
	for _, method := range service.Methods {
		genServerMethod(gen, file, g, method)
	}
}

func clientSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	s := method.GoName + "("
	s += "destination " + g.QualifiedGoIdent(kite.Ident("Destination"))
	if !method.Desc.IsStreamingClient() {
		s += ", request *" + g.QualifiedGoIdent(method.Input.GoIdent)
	}
	s += ", opts ..." + g.QualifiedGoIdent(kite.Ident("Option")) + ") ("
	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		s += "response *" + g.QualifiedGoIdent(method.Output.GoIdent)
	} else {
		s += method.Parent.GoName + "_" + method.GoName + "Client"
	}
	s += ", err error) "
	return s
}

func genClientMethod(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method, index int) {
	service := method.Parent
	// 获取文件名
	fileName := path.Base(file.GeneratedFilenamePrefix)
	if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
		g.P(deprecationComment)
	}
	g.P("func (c *", unExport(service.GoName), ") ", clientSignature(g, method), "{")
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		g.P("reqPBData, err := ", g.QualifiedGoIdent(proto.Ident("Marshal(request)")))
		g.P("if err != nil { return nil, ", g.QualifiedGoIdent(errors.Ident(`New("request marshal err")}`)))

		g.P(`resPBData, err := `, g.QualifiedGoIdent(kite.Ident("Invoke")), `(destination, "`, fileName, `", "`, export(service.GoName), `", "`, method.Desc.Name(), `", reqPBData, opts...)`)
		g.P("if err != nil { return nil, err }")
		g.P("response = new(", method.Output.GoIdent, ")")
		g.P("err = ", g.QualifiedGoIdent(proto.Ident("Unmarshal(resPBData, response)")))
		g.P("return")
		g.P("}")
		g.P()

		return
	}
}

func serverSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	var reqArgs []string
	ret := "error"
	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		ret = "(*" + g.QualifiedGoIdent(method.Output.GoIdent) + ", error)"
	}
	if !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "*"+g.QualifiedGoIdent(method.Input.GoIdent))
	}
	if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
		reqArgs = append(reqArgs, method.Parent.GoName+"_"+method.GoName+"Server")
	}
	reqArgs = append(reqArgs, "*kite.Destination")
	return method.GoName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func genServerMethod(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method) {
	service := method.Parent

	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		g.P("func (s *", export(service.GoName), "Service) ", method.GoName, "(reqPBData []byte, sender *kite.Destination) (resPBData []byte, err error) {")
		g.P("req := new(", method.Input.GoIdent, ")")
		g.P(g.QualifiedGoIdent(proto.Ident("Unmarshal(reqPBData, req)")))
		g.P("var res *", method.Output.GoIdent)
		g.P("res, err = s.handle.", method.GoName, "(req, sender)")
		g.P("if err == nil { resPBData, err = proto.Marshal(res) }")
		g.P("return")
		g.P("}")
		g.P("")
		return
	}
	return
}

const deprecationComment = "// Deprecated: Do not use."

func unExport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

func export(s string) string { return strings.ToUpper(s[:1]) + s[1:] }