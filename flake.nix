{
  description = "DMARC report parser and dashboard";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = fn: nixpkgs.lib.genAttrs systems (system: fn nixpkgs.legacyPackages.${system});
    in
    {
      formatter = forAllSystems (pkgs: pkgs.nixfmt-rfc-style);

      packages = forAllSystems (
        pkgs:
        let
          frontend = pkgs.buildNpmPackage {
            pname = "parse-dmarc-frontend";
            version = "0.0.0-dev";
            src = ./.;
            npmDepsHash = "sha256-6QQi2bqDx/AqMcBkNghcxYmqdYm+gdLq0YJkyudf7XQ=";
            nativeBuildInputs = with pkgs; [ python3 ];
            buildPhase = ''
              runHook preBuild
              npx vite build
              runHook postBuild
            '';
            installPhase = ''
              runHook preInstall
              cp -r dist $out
              runHook postInstall
            '';
          };
        in
        {
          default = pkgs.buildGoModule {
            pname = "parse-dmarc";
            version = "0.0.0-dev";
            src = ./.;
            vendorHash = "sha256-ojwyblK05W0O4GVVzKvsAfMc+EVWWBjn4F7RsT5S0/o=";
            env.CGO_ENABLED = 0;
            preBuild = ''
              cp -r ${frontend} internal/api/dist
            '';
            meta = {
              description = "DMARC report parser and dashboard";
              mainProgram = "parse-dmarc";
            };
          };
        }
      );

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages =
            with pkgs;
            [
              go
              goreleaser
              gotools
              golangci-lint
              just
              air
              bun
              nodejs
            ]
            ++ lib.optionals stdenv.isLinux [
              stdenv.cc.cc.lib
            ];
          env = pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {
            LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath [ pkgs.stdenv.cc.cc.lib ];
          };
        };
      });
    };
}
