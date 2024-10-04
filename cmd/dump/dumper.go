package dump

import (
    "encoding/json"
    "gopkg.in/yaml.v3"
    "fmt"
    "os"

    "github.com/moby/buildkit/client/llb"
    "github.com/moby/buildkit/solver/pb"
    digest "github.com/opencontainers/go-digest"
)

// DumpLLB outputs the LLB definition in the specified format
func DumpLLB(format string, def *llb.Definition) error {
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
        enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2) // Set indentation to 2 spaces
        for _, op := range ops {
            if err := enc.Encode(op); err != nil {
                return err
            }
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

