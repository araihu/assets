#!/usr/bin/env ruby
# Validates one tar member listing before extraction.

abort "usage: validate-release-archive-members.rb <member-list>" unless ARGV.length == 1

data = File.binread(ARGV.fetch(0)).force_encoding(Encoding::UTF_8)
abort "archive member list is not valid UTF-8" unless data.valid_encoding?

paths = data.lines(chomp: true)
abort "archive member list is empty" if paths.empty?

exact = {}
portable = {}
paths.each do |path|
  components = path.split("/", -1)
  abort "unsafe release archive member: #{path.inspect}" if
    path.empty? || path.start_with?("/") || path.include?("\\") ||
    components.include?("") || components.include?(".") || components.include?("..")

  abort "duplicate release archive member: #{path.inspect}" if exact.key?(path)
  exact[path] = true

  portable_path = path.unicode_normalize(:nfc).downcase(:fold)
  if (previous = portable[portable_path])
    abort "portable release archive member collision: #{previous.inspect} and #{path.inspect}"
  end
  portable[portable_path] = path
end
