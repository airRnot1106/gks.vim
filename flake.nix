{
  description = "Romaji search for Japanese text in Vim and Neovim on goda.vim";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    goda-vim = {
      url = "github:Shougo/goda.vim/v0.7.2";
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      goda-vim,
    }:
    let
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      version = "0.1.0";

      mkPackages = pkgs: rec {
        # GPL, because it is made from the SKK dictionaries. It stays a data file
        # next to the binary, and is not part of this repository.
        dictionary = pkgs.fetchurl {
          url = "https://github.com/oguna/migemo-compact-dict-latest/releases/download/v0.2/migemo-compact-dict";
          hash = "sha256-+jrCQLM1iSIvzUCRDe0sFpc3TgGvFNwrtd32g6oumho=";
        };

        goda-ks = pkgs.buildGo126Module {
          pname = "goda-ks";
          inherit version;
          src = ./.;
          vendorHash = "sha256-c7GwOs1wzh21OXmzONbaCCXbwcK95LbWmcS+x6qdE9M=";
          subPackages = [ "." ];
          doCheck = false;
          postInstall = ''
            mv $out/bin/goda-ks.vim $out/bin/goda-ks
            install -Dm444 ${dictionary} $out/share/goda-ks/migemo-compact-dict
          '';
          meta.mainProgram = "goda-ks";
        };

        goda-vim-plugin = pkgs.vimUtils.buildVimPlugin {
          pname = "goda.vim";
          version = "0-${goda-vim.shortRev}";
          src = goda-vim;
        };

        vim-plugin = pkgs.vimUtils.buildVimPlugin {
          pname = "goda-ks.vim";
          inherit version;
          src = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./LICENSE
              ./autoload
              ./doc
              ./plugin
            ];
          };
          dependencies = [ goda-vim-plugin ];
          preInstall = ''
            cp -r ${goda-ks}/bin ${goda-ks}/share .
          '';
        };

        default = vim-plugin;
      };
    in
    {
      packages = forAllSystems (
        pkgs:
        let
          built = mkPackages pkgs;
        in
        {
          inherit (built)
            dictionary
            goda-ks
            vim-plugin
            default
            ;
          goda-vim = built.goda-vim-plugin;
        }
      );

      checks = forAllSystems (
        pkgs:
        let
          built = mkPackages pkgs;
          goTest =
            name: command:
            built.goda-ks.overrideAttrs {
              pname = "goda-ks-${name}";
              doCheck = true;
              checkPhase = ''
                runHook preCheck
                export GODA_KS_TEST_DICT=${built.dictionary}
                ${command}
                runHook postCheck
              '';
              installPhase = "touch $out";
              postInstall = "";
            };
        in
        {
          unit = goTest "unit" "go test ./internal/... ./migemo/...";
          e2e = goTest "e2e" ''
            export HOME=$TMPDIR
            export PATH=${
              pkgs.lib.makeBinPath [
                pkgs.vim
                pkgs.neovim
              ]
            }:$PATH
            export GODA_KS_E2E_GODA=${goda-vim}
            export GODA_KS_E2E_STRICT=1
            go test -v -count=1 ./e2e
          '';
        }
      );

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go_1_26
            pkgs.vim
            pkgs.neovim
          ];
        };
      });
    };
}
