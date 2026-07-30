#!/usr/bin/env ruby
# Computes the v1 digest of the complete, closed channel bundle.
require "digest"
require "find"

ALLOWED_PATHS = [
  "campaign/v1.js",
  "releases/latest.json",
  "releases/default.json",
  "releases/current.json"
].freeze

abort "usage: channel-bundle-digest.rb <bundle-directory>" unless ARGV.length == 1
root = File.expand_path(ARGV.fetch(0))
abort "bundle root is not a directory" unless File.lstat(root).directory?

actual = []
Find.find(root) do |absolute|
  next if absolute == root
  stat = File.lstat(absolute)
  relative = absolute.delete_prefix(root + File::SEPARATOR).tr(File::SEPARATOR, "/")
  if stat.directory?
    next
  elsif stat.file?
    actual << relative
  else
    abort "bundle contains non-regular path: #{relative}"
  end
end
actual.sort!
abort "bundle paths differ from closed v1 contract" unless actual == ALLOWED_PATHS.sort

digest = Digest::SHA256.new
digest << "araihu-channel-bundle-v1\0"
ALLOWED_PATHS.each do |path|
  data = File.binread(File.join(root, path))
  digest << [path.bytesize].pack("Q>")
  digest << path
  digest << [data.bytesize].pack("Q>")
  digest << data
end
puts digest.hexdigest
