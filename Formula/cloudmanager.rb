class Cloudmanager < Formula
  desc "Fast terminal control plane for cloud operations"
  homepage "https://github.com/srivathsan-srinivasan/cloudmanager"
  url "https://github.com/srivathsan-srinivasan/cloudmanager.git",
      tag: "v1.0.1"
  version "1.0.1"
  license "MIT"
  head "https://github.com/srivathsan-srinivasan/cloudmanager.git",
       branch: "release/v1.0.0"

  depends_on "go" => :build

  def install
    build_time = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%SZ")
    ldflags = "-s -w -X main.Version=#{version} -X main.BuildTime=#{build_time}"
    system "go", "build", "-trimpath", "-ldflags", ldflags, "-o", bin/"cloudmanager", "."
  end

  test do
    assert_match "cloudmanager v#{version}", shell_output("#{bin}/cloudmanager --version")
  end
end
