package main

import (
	"flag"
	"fmt"
	"path"

	"github.com/franchb/protoc-gen-nats-micro/tools/protoc-gen-nats-micro/generator"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const version = "0.3.0"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-nats-micro %v\n", version)
		return
	}

	// Plugin parameters (e.g., --nats-micro_opt=language=typescript) are parsed
	// through this flag set so that unknown parameters fail loudly instead of
	// being silently ignored.
	var flags flag.FlagSet
	language := flags.String("language", "go", "target language (go, python, typescript, web-ts)")
	flags.Func("lang", "alias for -language", func(v string) error {
		*language = v
		return nil
	})

	protogen.Options{ParamFunc: flags.Set}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		// Resolve language once — used for all files
		lang, err := generator.GetLanguage(*language)
		if err != nil {
			return fmt.Errorf("get language: %w", err)
		}

		// Track which packages have had shared files generated
		generatedShared := make(map[string]bool)

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}

			// pkgDir is "." for root-level protos, so path.Join below keeps
			// the shared file next to the per-file outputs.
			filenameBase := generator.OutputFilenamePrefix(f, lang)
			pkgDir := path.Dir(filenameBase)

			// For Go-like: key by the import path (e.g., "github.com/example/gen/order/v1")
			// For others: key by the directory path (e.g., "gen/order/v1")
			pkgKey := string(f.GoImportPath)
			if !lang.IsGoLike() {
				pkgKey = pkgDir
			}

			if generator.HasGeneratableServices(f) && !generatedShared[pkgKey] {
				generatedShared[pkgKey] = true

				sharedPrefix := path.Join(pkgDir, "shared")

				// A proto file literally named shared.proto would produce the
				// same output path as the per-package shared file; fail with a
				// clear message instead of protoc's "Tried to write the same
				// file twice".
				for _, other := range gen.Files {
					if other.Generate && generator.HasGeneratableServices(other) &&
						generator.OutputFilenamePrefix(other, lang) == sharedPrefix {
						return fmt.Errorf(
							"proto file %s would generate %s%s, which collides with the per-package shared file; rename the proto file",
							other.Desc.Path(), sharedPrefix, lang.FileExtension(),
						)
					}
				}

				sharedFile := gen.NewGeneratedFile(sharedPrefix+lang.FileExtension(), generator.ImportPathFor(f, lang))

				// Generate shared content through the Language interface
				if err := lang.GenerateShared(sharedFile, f); err != nil {
					return fmt.Errorf("generate shared: %w", err)
				}

				// Allow language-specific post-generation (e.g., Python __init__.py)
				if err := lang.PostGenerate(gen, f, pkgDir); err != nil {
					return fmt.Errorf("post generate: %w", err)
				}
			}

			if err := generator.GenerateFile(gen, f, lang); err != nil {
				return fmt.Errorf("generate file %s: %w", f.Desc.Path(), err)
			}
		}
		return nil
	})
}
