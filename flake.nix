{
  description = "A small Cloudflare dynamic DNS client";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      pkgsFor = system: import nixpkgs { inherit system; };
      flareFor =
        system:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = "flare";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "cmd/flare" ];
          vendorHash = null;
          meta = {
            description = "Update a Cloudflare DNS record with the current public IP address";
            homepage = "https://github.com/marksisson/flare";
            mainProgram = "flare";
            platforms = nixpkgs.lib.platforms.unix;
          };
        };
    in
    {
      packages = forAllSystems (system: {
        default = flareFor system;
        flare = flareFor system;
      });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${flareFor system}/bin/flare";
        };
      });

      checks = forAllSystems (system: {
        flare = flareFor system;
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = pkgsFor system;
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
            ];
          };
        }
      );

      formatter = forAllSystems (system: (pkgsFor system).nixfmt);
    };
}
