package dump

import (
    "encoding/json"
    "bytes"
    "fmt"
    "os"
	"io"
	"strings"

    "golang.org/x/term"
    "gopkg.in/yaml.v3"

    "github.com/alecthomas/chroma/v2"
    "github.com/alecthomas/chroma/v2/formatters"
    "github.com/alecthomas/chroma/v2/lexers"
    "github.com/moby/buildkit/client/llb"
    "github.com/moby/buildkit/solver/pb"
    digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

)




var customStyle = chroma.MustNewStyle("myCustomStyle", chroma.StyleEntries{
	chroma.Error: "#a61717 bg:#e3d2d2",
	chroma.Background: "bg:#000000",
	chroma.Keyword: "#ffffff",
	chroma.KeywordType: "bold #445588",
	chroma.NameAttribute: "#008080",
	chroma.NameBuiltin: "#0086b3",
	chroma.NameBuiltinPseudo: "#999999",
	chroma.NameClass: "bold #445588",
	chroma.NameConstant: "#008080",
	chroma.NameDecorator: "bold #3c5d5d",
	chroma.NameEntity: "#800080",
	chroma.NameException: "bold #990000",
	chroma.NameFunction: "bold #990000",
	chroma.NameLabel: "bold #990000",
	chroma.NameNamespace: "#555555",
	chroma.NameTag: "#000080",
	chroma.NameVariable: "#008080",
	chroma.NameVariableClass: "#008080",
	chroma.NameVariableGlobal: "#008080",
	chroma.NameVariableInstance: "#008080",
	chroma.LiteralString: "#88bb88",
	chroma.LiteralStringRegex: "#009926",
	chroma.LiteralStringSymbol: "#990073",
	chroma.LiteralNumber: "#009999",
	chroma.Literal: "#88bb88",
	chroma.Operator: "bold #ffffff",
	chroma.Comment: "italic #999988",
	chroma.CommentMultiline: "italic #999988",
	chroma.CommentSingle: "italic #999988",
	chroma.CommentSpecial: "bold italic #999999",
	chroma.CommentPreproc: "bold #999999",
	chroma.GenericDeleted: "#000000 bg:#ffdddd",
	chroma.GenericEmph: "italic #ffffff",
	chroma.GenericError: "#aa0000",
	chroma.GenericHeading: "#999999",
	chroma.GenericInserted: "#000000 bg:#ddffdd",
	chroma.GenericOutput: "#888888",
	chroma.GenericPrompt: "#555555",
	chroma.GenericStrong: "bold",
	chroma.GenericSubheading: "#aaaaaa",
	chroma.GenericTraceback: "#aa0000",
	chroma.GenericUnderline: "underline",
	chroma.TextWhitespace: "#bbbbbb",
})

// DumpLLB outputs the LLB definition in the specified format
func DumpLLB(format string, def *llb.Definition, color bool) error {
    switch format {
    case "llb":
        return llb.WriteTo(def, os.Stdout)
    case "dot":
        ops, err := loadLLB(def)
        if err != nil {
            return err
        }
        writeDot(ops, os.Stdout)
    case "json":
        ops, err := loadLLB(def)
        if err != nil {
            return err
        }
        enc := json.NewEncoder(os.Stdout)
        for _, op := range ops {
            if err := enc.Encode(op); err != nil {
                return err
            }
        }
    case "yaml":
        ops, err := loadLLB(def)
        if err != nil {
            return err
        }
        var buf bytes.Buffer
        enc := yaml.NewEncoder(&buf)
        enc.SetIndent(2)
        for _, op := range ops {
            if err := enc.Encode(op); err != nil {
                return err
            }
        }
        enc.Close()

        // Detect if output is a terminal and enable color if so
        if !color {
            color = term.IsTerminal(int(os.Stdout.Fd()))
        }

		if color {
			// Initialize the lexer for YAML
			lexer := lexers.Get("yaml")
			if lexer == nil {
				return fmt.Errorf("no lexer found for yaml")
			}
			lexer = chroma.Coalesce(lexer)

			// Initialize the formatter
			formatter := formatters.Get("terminal256")
			if formatter == nil {
				return fmt.Errorf("no formatter found for terminal256")
			}

			// Tokenize the YAML content
			iterator, err := lexer.Tokenise(nil, buf.String())
			if err != nil {
				return err
			}

			// Format and output the highlighted content
			return formatter.Format(os.Stdout, customStyle, iterator)
		} else {
			_, err = os.Stdout.Write(buf.Bytes())
			return err
		}
    default:
        return fmt.Errorf("unknown format: %s", format)
    }
    return nil
}

func loadLLB(def *llb.Definition) ([]llbOp, error) {
        var ops []llbOp
        for _, dt := range def.Def {
                var op pb.Op
                if err := proto.Unmarshal(dt, &op); err != nil {
                        return nil, errors.Wrap(err, "failed to parse op")
                }
                dgst := digest.FromBytes(dt)
                ent := llbOp{Op: &op, Digest: dgst, OpMetadata: def.Metadata[dgst].ToPB()}
                ops = append(ops, ent)
        }
        return ops, nil
}

type llbOp struct {
    Op         *pb.Op
    Digest     digest.Digest
    OpMetadata *pb.OpMetadata
}


func writeDot(ops []llbOp, w io.Writer) {
	// TODO: print OpMetadata
	fmt.Fprintln(w, "digraph {")
	defer fmt.Fprintln(w, "}")
	for _, op := range ops {
		name, shape := attr(op.Digest, *op.Op)
		fmt.Fprintf(w, "  %q [label=%q shape=%q];\n", op.Digest, name, shape)
	}
	for _, op := range ops {
		for i, inp := range op.Op.Inputs {
			label := ""
			if eo, ok := op.Op.Op.(*pb.Op_Exec); ok {
				for _, m := range eo.Exec.Mounts {
					if int(m.Input) == i && m.Dest != "/" {
						label = m.Dest
					}
				}
			}
			fmt.Fprintf(w, "  %q -> %q [label=%q];\n", inp.Digest, op.Digest, label)
		}
	}
}

func attr(dgst digest.Digest, op pb.Op) (string, string) {
	switch op := op.Op.(type) {
	case *pb.Op_Source:
		return op.Source.Identifier, "ellipse"
	case *pb.Op_Exec:
		return strings.Join(op.Exec.Meta.Args, " "), "box"
	case *pb.Op_Build:
		return "build", "box3d"
	case *pb.Op_Merge:
		return "merge", "invtriangle"
	case *pb.Op_Diff:
		return "diff", "doublecircle"
	case *pb.Op_File:
		names := []string{}

		for _, action := range op.File.Actions {
			var name string

			switch act := action.Action.(type) {
			case *pb.FileAction_Copy:
				name = fmt.Sprintf("copy{src=%s, dest=%s}", act.Copy.Src, act.Copy.Dest)
			case *pb.FileAction_Mkfile:
				name = fmt.Sprintf("mkfile{path=%s}", act.Mkfile.Path)
			case *pb.FileAction_Mkdir:
				name = fmt.Sprintf("mkdir{path=%s}", act.Mkdir.Path)
			case *pb.FileAction_Rm:
				name = fmt.Sprintf("rm{path=%s}", act.Rm.Path)
			}

			names = append(names, name)
		}
		return strings.Join(names, ","), "note"
	default:
		return dgst.String(), "plaintext"
	}
}
