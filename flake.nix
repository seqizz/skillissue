{
  description = "skillissue - manage LLM agent skills across git repos and agent skill dirs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "skillissue";
          version = "0.1.1";
          src = ./.;
          # Deps are vendored in ./vendor, so no network fetch / vendor hash needed.
          vendorHash = null;

          # skillissue shells out to the git binary at runtime.
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/skillissue \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git ]}
          '';

          meta = with pkgs.lib; {
            description = "Manage LLM agent skills across git repos and agent skill dirs";
            mainProgram = "skillissue";
            license = licenses.wtfpl;
            platforms = platforms.unix;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.git
          ];
        };
      }
    );
}
