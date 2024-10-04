package dump

import (
    "encoding/json"
    "bytes"
    "fmt"
    "os"

    "golang.org/x/term"
    "gopkg.in/yaml.v3"

    "github.com/alecthomas/chroma/v2/quick"
    "github.com/moby/buildkit/client/llb"
    "github.com/moby/buildkit/solver/pb"
    digest "github.com/opencontainers/go-digest"
)

// DumpLLB outputs the LLB definition in the specified format
func DumpLLB(format string, def *llb.Definition, color bool) error {
    switch format {
    case "llb":
        return llb.WriteTo(def, os.Stdout)
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
            // Use chroma to highlight YAML syntax
            return quick.Highlight(os.Stdout, buf.String(), "yaml", "terminal256", "github")
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
        if err := (&op).Unmarshal(dt); err != nil {
            return nil, fmt.Errorf("failed to parse op: %w", err)
        }
        dgst := digest.FromBytes(dt)
        ent := llbOp{Op: op, Digest: dgst, OpMetadata: def.Metadata[dgst]}
        ops = append(ops, ent)
    }
    return ops, nil
}

type llbOp struct {
    Op         pb.Op
    Digest     digest.Digest
    OpMetadata pb.OpMetadata
}

