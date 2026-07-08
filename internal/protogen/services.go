package protogen

import "google.golang.org/protobuf/reflect/protoreflect"

// Service is a single proto `service` with its routable (non-streaming) RPCs.
type Service struct {
	Name             string // short Go name, e.g. "TodoService"
	FQName           string // fully-qualified, e.g. "todo.v1.TodoService"
	Methods          []Method
	SkippedStreaming []string
}

// Method is a single routable RPC.
type Method struct {
	Name         string // RPC name, e.g. "GetTodo"
	RequestType  string // bare model type name, "" if the request is google.protobuf.Empty
	ResponseType string // bare model type name, "" if the response is google.protobuf.Empty
	Route        Route
}

// buildServices converts every service declared in fd into the IR, skipping
// streaming RPCs (collected separately so callers can note them).
func buildServices(fd protoreflect.FileDescriptor) []Service {
	var services []Service
	svcs := fd.Services()
	for i := 0; i < svcs.Len(); i++ {
		sd := svcs.Get(i)
		svc := Service{Name: string(sd.Name()), FQName: string(sd.FullName())}

		methods := sd.Methods()
		for j := 0; j < methods.Len(); j++ {
			md := methods.Get(j)
			if md.IsStreamingClient() || md.IsStreamingServer() {
				svc.SkippedStreaming = append(svc.SkippedStreaming, string(md.Name()))
				continue
			}

			respType, _ := topLevelTypeRef(md.Output())
			route := resolveRoute(md, svc.FQName)
			if _, reqEmpty := topLevelTypeRef(md.Input()); reqEmpty {
				route.BodyField = ""
			}

			svc.Methods = append(svc.Methods, Method{
				Name:         string(md.Name()),
				RequestType:  requestBodyType(md.Input(), route.BodyField),
				ResponseType: respType,
				Route:        route,
			})
		}

		if len(svc.Methods) > 0 || len(svc.SkippedStreaming) > 0 {
			services = append(services, svc)
		}
	}
	return services
}

// requestBodyType resolves the Go model type actually sent as the request
// body, per bodyField ("" = no body -> no @SnAPI.Request at all; "*" = the
// whole input message; a field name = google.api.http's `body: "field"`,
// meaning only that field's (message-typed) value is the JSON body).
func requestBodyType(input protoreflect.MessageDescriptor, bodyField string) string {
	switch bodyField {
	case "":
		return ""
	case "*":
		t, empty := topLevelTypeRef(input)
		if empty {
			return ""
		}
		return t
	default:
		f := input.Fields().ByName(protoreflect.Name(bodyField))
		if f == nil || f.Kind() != protoreflect.MessageKind {
			// Not representable as a named model type; fall back to the
			// whole input message rather than silently dropping the body.
			t, empty := topLevelTypeRef(input)
			if empty {
				return ""
			}
			return t
		}
		return flattenedMessageName(f.Message())
	}
}
