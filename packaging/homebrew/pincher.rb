# Homebrew formula for pincherMCP.
#
# Pinned to v0.35.0. The SHA256 values below are from the authoritative
# SHA256SUMS file published with that release:
# https://github.com/kwad77/pincher/releases/download/v0.35.0/SHA256SUMS
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
  version "0.35.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-darwin-arm64.tar.gz"
      sha256 "f53cd6329106f4b7ffd9b6e08bb822dbb896cfe45fe4bfd5ada23c02cae5932d"
    end
    on_intel do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-darwin-amd64.tar.gz"
      sha256 "4a20ad8777cb2cfa05f6b869b2a88dea40372e2a6df03a16a10c57811b153a35"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-linux-arm64.tar.gz"
      sha256 "af6ca113a5cdbfbc6bd9316ddd63e6142b91df4384d0d5213fb380c85fb3ae07"
    end
    on_intel do
      url "https://github.com/kwad77/pincher/releases/download/v#{version}/pincher-v#{version}-linux-amd64.tar.gz"
      sha256 "f9edfed638eae771cdaab82301e861cb6f7b6d2fd90e5944b54943cdca80b43b"
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
