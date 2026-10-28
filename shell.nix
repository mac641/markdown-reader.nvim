# Development shell with everything `make`, `make test` and the linters
# need:
#
#   nix-shell                 # then make, make test, ...
#   nix-shell --run make      # just build bin/mdreader
{
  pkgs ? import <nixpkgs> { },
}:

pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    go
    gnumake
    pkg-config
    neovim
    golangci-lint
    shellcheck
    deno
  ];
  # macOS uses the system's WebKit, which the default SDK provides.
  buildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux (
    with pkgs;
    [
      gtk4
      webkitgtk_6_0
    ]
  );
}
