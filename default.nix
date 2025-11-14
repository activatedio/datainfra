with import <nixpkgs> {};

stdenv.mkDerivation {
  name = "datainfra";
  buildInputs = with pkgs; [
    go
    gnumake
    postgresql
  ];
  shellHook = ''
    export GOPATH=$HOME/go
    export PATH=$PATH:$GOPATH/bin
  '';
  hardeningDisable = [ "fortify" ];
}
