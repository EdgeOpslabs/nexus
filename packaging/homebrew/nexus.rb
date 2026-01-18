class Nexus < Formula
  desc "Nexus - the unified bridge for infrastructure context"
  homepage "https://github.com/edgeopslabs/nexus-core"
  url "https://github.com/edgeopslabs/nexus-core/releases/download/v0.0.1/nexus_darwin_amd64.tar.gz"
  sha256 "REPLACE_WITH_SHA256"
  version "0.0.1"

  def install
    bin.install "nexus"
  end

  test do
    system "#{bin}/nexus", "--help"
  end
end
