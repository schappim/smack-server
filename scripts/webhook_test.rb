#!/usr/bin/env ruby
require 'httparty'
require 'json'
require 'sqlite3'

# Configuration
BASE_URL = ENV['SMACK_URL'] || 'http://localhost:8080'
DB_PATH = File.expand_path('../smack.db', __dir__)

# Connect to database
db = SQLite3::Database.new(DB_PATH)
db.results_as_hash = true

# Check for existing webhooks
webhooks = db.execute("SELECT id, name, channel_id, token FROM webhooks")

if webhooks.empty?
  puts "No webhooks found. Creating one..."

  # Get channels
  puts "\nAvailable channels:"
  channels = db.execute("SELECT id, name, is_direct FROM channels WHERE is_direct = 0 ORDER BY name")
  channels.each_with_index do |ch, i|
    puts "  #{i + 1}. #{ch['name']} - #{ch['id']}"
  end

  print "\nSelect channel number: "
  input = gets.chomp
  channel = channels[input.to_i - 1]

  puts "\nYou need to create a webhook via the API with auth first."
  puts "Or insert directly into DB for testing:"
  puts "\n  sqlite3 #{DB_PATH} \"INSERT INTO webhooks (id, name, channel_id, token, created_by) VALUES ('test-webhook', 'Test Webhook', '#{channel['id']}', 'secret-token', 'system');\""
  exit 1
end

puts "Available webhooks:"
webhooks.each_with_index do |wh, i|
  channel = db.execute("SELECT name FROM channels WHERE id = ?", wh['channel_id']).first
  puts "  #{i + 1}. #{wh['name']} -> ##{channel['name']} (#{wh['id']})"
end

print "\nSelect webhook number: "
input = gets.chomp
webhook = webhooks[input.to_i - 1]

print "Message: "
message = gets.chomp
message = "Hello from webhook!" if message.empty?

# Send webhook request
url = "#{BASE_URL}/api/webhooks/incoming/#{webhook['id']}/#{webhook['token']}"
puts "\nSending to #{url}..."
sleep 2

response = HTTParty.post(
  url,
  headers: { 'Content-Type' => 'application/json' },
  body: {
    content: message,
    username: 'Webhook Bot'
  }.to_json
)

puts "Status: #{response.code}"
puts "Response: #{response.body}"
