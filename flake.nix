{
  description = "Piledriver dev environment - TLA+ bug hunting";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Create wrapper scripts that work with --command
        tlc = pkgs.writeShellScriptBin "tlc" ''
          TLA2TOOLS="''${TLA2TOOLS:-$PWD/tools/tla2tools.jar}"

          # Find the spec file to determine output directory
          SPEC_FILE=""
          for arg in "$@"; do
            if [[ "$arg" == *.tla ]]; then
              SPEC_FILE="$arg"
              break
            fi
          done

          if [[ -n "$SPEC_FILE" ]]; then
            SPEC_DIR="$(dirname "$SPEC_FILE")"
            OUTDIR="$SPEC_DIR/_tlc_out"
            mkdir -p "$OUTDIR"
            ${pkgs.openjdk17}/bin/java -jar "$TLA2TOOLS" -metadir "$OUTDIR" "$@"
            EXIT_CODE=$?
            # Move trace files to output dir
            mv *_TTrace_*.tla *_TTrace_*.bin states/ "$OUTDIR" 2>/dev/null || true
            exit $EXIT_CODE
          else
            exec ${pkgs.openjdk17}/bin/java -jar "$TLA2TOOLS" "$@"
          fi
        '';

        sany = pkgs.writeShellScriptBin "sany" ''
          TLA2TOOLS="''${TLA2TOOLS:-$PWD/tools/tla2tools.jar}"
          exec ${pkgs.openjdk17}/bin/java -cp "$TLA2TOOLS" tla2sany.SANY "$@"
        '';
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.openjdk17
            pkgs.git
            tlc
            sany
          ];

          shellHook = ''
            export TLA2TOOLS="$PWD/tools/tla2tools.jar"
            echo "Piledriver dev environment loaded"
            echo "  tlc <spec.tla>  - Run TLC model checker"
            echo "  sany <spec.tla> - Check TLA+ syntax"
            echo ""
            java -version 2>&1 | head -1
          '';
        };
      });
}
