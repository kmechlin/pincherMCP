# Homebrew formula for pincherMCP.
#
# Pinned to v0.26.0. The SHA256 values below are from the authoritative
# SHA256SUMS file published with that release:
# https://github.com/kwad77/pincher/releases/download/v0.26.0/SHA256SUMS
#
# Usage:
#   brew tap kwad77/pincher https://github.com/kwad77/homebrew-pincher
#   brew install pincher
#
# To host the tap yourself, create a repo named "homebrew-pincher" under
# your GitHub account and drop this file in at Formula/pincher.rb.
#
# On each new release: bump `version`, refetch the release's SHA256SUMS,
# and paste the four new Darwin/Linux (arm64/amd64) hashes into the
# sha256 lines below.
class Pincher < Formula
  desc "Codebase intelligence server for LLM agents (MCP stdio + HTTP REST)"
  homepage "https://github.com/kwad77/pincher"
  version "0.26.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-darwin-arm64.tar.gz"
      sha256 "0477bf9b042a5f2f28a4c37c42d0a3b7b8283ee44e962531f1b49d575485150c"
    end
    on_intel do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-darwin-amd64.tar.gz"
      sha256 "10c8cf5cd94dcfdff67dba07d588e89990cc7bfdf27591aeda0f2ceb44b58e9a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-linux-arm64.tar.gz"
      sha256 "460555c2858603cef25eb0ff5a948216b45b080bacb4aeb14e8c01eb0d8afda8"
    end
    on_intel do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-linux-amd64.tar.gz"
      sha256 "f904b4b7127836382a1a95757b6271954b35d7a0eb01380e5e02dcdb12c195ea"
    end
  end

  def install
    # Archives contain one file: pincher-v<version>-<os>-<arch>[.exe]
    bin_src = Dir["pincher-*"].first
    bin.install bin_src => "pincher"
  end

  test do
    assert_match "pincherMCP", shell_output("#{bin}/pincher --version")
  end

  service do
    run [opt_bin/"pincher", "--http", ":8080"]
    keep_alive true
    log_path var/"log/pincher.log"
    error_log_path var/"log/pincher.err.log"
    environment_variables PINCHER_HTTP_ADDR: ":8080"
  end
end
