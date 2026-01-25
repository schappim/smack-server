#!/usr/bin/env ruby
require 'httparty'
require 'json'
require 'sqlite3'

# Configuration
BASE_URL = ENV['SMACK_URL'] || 'http://localhost:8080'
DB_PATH = File.expand_path('../smack.db', __dir__)

# Widget sizes: small (100x280), medium (150x320), large (200x380), xlarge (300x450)

# Sample HTML widgets
WIDGETS = {
  'weather' => {
    content: 'Weather Update',
    username: 'Weather Bot',
    widget_size: 'large',
    html: <<~HTML
      <div style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 12px; font-family: -apple-system, sans-serif;">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div>
            <div style="font-size: 48px; font-weight: 300;">72°F</div>
            <div style="opacity: 0.9;">San Francisco, CA</div>
          </div>
          <div style="font-size: 48px;">☀️</div>
        </div>
        <div style="margin-top: 16px; padding-top: 16px; border-top: 1px solid rgba(255,255,255,0.2); display: flex; justify-content: space-between;">
          <div style="text-align: center;"><div style="opacity: 0.7; font-size: 12px;">Mon</div><div>68°</div></div>
          <div style="text-align: center;"><div style="opacity: 0.7; font-size: 12px;">Tue</div><div>71°</div></div>
          <div style="text-align: center;"><div style="opacity: 0.7; font-size: 12px;">Wed</div><div>75°</div></div>
          <div style="text-align: center;"><div style="opacity: 0.7; font-size: 12px;">Thu</div><div>73°</div></div>
          <div style="text-align: center;"><div style="opacity: 0.7; font-size: 12px;">Fri</div><div>70°</div></div>
        </div>
      </div>
    HTML
  },
  'stats' => {
    content: 'Daily Stats',
    username: 'Stats Bot',
    widget_size: 'large',
    html: <<~HTML
      <div style="background: #1a1a2e; color: white; padding: 20px; border-radius: 12px; font-family: -apple-system, sans-serif;">
        <div style="font-size: 14px; opacity: 0.7; margin-bottom: 12px;">PROJECT METRICS</div>
        <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px;">
          <div style="background: #16213e; padding: 16px; border-radius: 8px; text-align: center;">
            <div style="font-size: 28px; font-weight: 600; color: #4ade80;">247</div>
            <div style="font-size: 12px; opacity: 0.7;">Commits</div>
          </div>
          <div style="background: #16213e; padding: 16px; border-radius: 8px; text-align: center;">
            <div style="font-size: 28px; font-weight: 600; color: #60a5fa;">18</div>
            <div style="font-size: 12px; opacity: 0.7;">PRs Merged</div>
          </div>
          <div style="background: #16213e; padding: 16px; border-radius: 8px; text-align: center;">
            <div style="font-size: 28px; font-weight: 600; color: #f472b6;">3</div>
            <div style="font-size: 12px; opacity: 0.7;">Issues</div>
          </div>
        </div>
      </div>
    HTML
  },
  'alert' => {
    content: 'System Alert',
    username: 'Alert Bot',
    widget_size: 'small',
    html: <<~HTML
      <div style="background: linear-gradient(135deg, #ff6b6b 0%, #ee5a5a 100%); color: white; padding: 16px 20px; border-radius: 12px; font-family: -apple-system, sans-serif; display: flex; align-items: center; gap: 12px;">
        <div style="font-size: 24px;">⚠️</div>
        <div>
          <div style="font-weight: 600;">High CPU Usage Detected</div>
          <div style="font-size: 13px; opacity: 0.9;">Server prod-web-01 is at 94% CPU utilization</div>
        </div>
      </div>
    HTML
  },
  'deploy' => {
    content: 'Deployment Complete',
    username: 'Deploy Bot',
    widget_size: 'medium',
    html: <<~HTML
      <div style="background: #0d1117; color: #c9d1d9; padding: 16px; border-radius: 12px; font-family: ui-monospace, monospace; font-size: 13px; border: 1px solid #30363d;">
        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 12px;">
          <span style="color: #3fb950;">✓</span>
          <span style="font-weight: 600;">Deploy succeeded</span>
          <span style="background: #238636; color: white; padding: 2px 8px; border-radius: 12px; font-size: 11px;">production</span>
        </div>
        <div style="background: #161b22; padding: 12px; border-radius: 6px;">
          <div><span style="color: #8b949e;">Branch:</span> main</div>
          <div><span style="color: #8b949e;">Commit:</span> a1b2c3d</div>
          <div><span style="color: #8b949e;">Duration:</span> 2m 34s</div>
        </div>
      </div>
    HTML
  },
  'quote' => {
    content: 'Daily Inspiration',
    username: 'Quote Bot',
    widget_size: 'large',
    html: <<~HTML
      <div style="background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%); color: #333; padding: 24px; border-radius: 12px; font-family: Georgia, serif;">
        <div style="font-size: 32px; color: #667eea; margin-bottom: 8px;">"</div>
        <div style="font-size: 18px; font-style: italic; line-height: 1.6;">The only way to do great work is to love what you do.</div>
        <div style="margin-top: 16px; font-size: 14px; color: #666;">— Steve Jobs</div>
      </div>
    HTML
  },
  'poll' => {
    content: 'Quick Poll',
    username: 'Poll Bot',
    widget_size: 'xlarge',
    html: <<~HTML
      <div style="background: #ffffff; color: #333; padding: 20px; border-radius: 12px; font-family: -apple-system, sans-serif; box-shadow: 0 2px 8px rgba(0,0,0,0.1);">
        <div style="font-weight: 600; margin-bottom: 16px;">What's your favorite programming language?</div>
        <div id="poll-options">
          <button onclick="vote('rust')" style="display: block; width: 100%; padding: 12px; margin-bottom: 8px; border: 2px solid #e0e0e0; border-radius: 8px; background: white; cursor: pointer; text-align: left; font-size: 14px; transition: all 0.2s;" onmouseover="this.style.borderColor='#667eea'" onmouseout="this.style.borderColor='#e0e0e0'">
            🦀 Rust
          </button>
          <button onclick="vote('python')" style="display: block; width: 100%; padding: 12px; margin-bottom: 8px; border: 2px solid #e0e0e0; border-radius: 8px; background: white; cursor: pointer; text-align: left; font-size: 14px; transition: all 0.2s;" onmouseover="this.style.borderColor='#667eea'" onmouseout="this.style.borderColor='#e0e0e0'">
            🐍 Python
          </button>
          <button onclick="vote('typescript')" style="display: block; width: 100%; padding: 12px; margin-bottom: 8px; border: 2px solid #e0e0e0; border-radius: 8px; background: white; cursor: pointer; text-align: left; font-size: 14px; transition: all 0.2s;" onmouseover="this.style.borderColor='#667eea'" onmouseout="this.style.borderColor='#e0e0e0'">
            📘 TypeScript
          </button>
          <button onclick="vote('go')" style="display: block; width: 100%; padding: 12px; border: 2px solid #e0e0e0; border-radius: 8px; background: white; cursor: pointer; text-align: left; font-size: 14px; transition: all 0.2s;" onmouseover="this.style.borderColor='#667eea'" onmouseout="this.style.borderColor='#e0e0e0'">
            🐹 Go
          </button>
        </div>
        <div id="poll-result" style="display: none; text-align: center; padding: 20px; color: #667eea;">
          <div style="font-size: 24px; margin-bottom: 8px;">✓</div>
          <div style="font-weight: 600;">Thanks for voting!</div>
        </div>
        <script>
          function vote(choice) {
            document.getElementById('poll-options').style.display = 'none';
            document.getElementById('poll-result').style.display = 'block';
          }
        </script>
      </div>
    HTML
  },
  'counter' => {
    content: 'Interactive Counter',
    username: 'Widget Bot',
    widget_size: 'large',
    html: <<~HTML
      <div style="background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%); color: white; padding: 24px; border-radius: 12px; font-family: -apple-system, sans-serif; text-align: center;">
        <div style="font-size: 14px; opacity: 0.7; margin-bottom: 8px;">COUNTER</div>
        <div id="count" style="font-size: 48px; font-weight: 700; margin-bottom: 16px;">0</div>
        <div style="display: flex; gap: 12px; justify-content: center;">
          <button onclick="decrement()" style="width: 48px; height: 48px; border-radius: 50%; border: none; background: #ff6b6b; color: white; font-size: 24px; cursor: pointer; transition: transform 0.1s;" onmousedown="this.style.transform='scale(0.95)'" onmouseup="this.style.transform='scale(1)'">−</button>
          <button onclick="increment()" style="width: 48px; height: 48px; border-radius: 50%; border: none; background: #4ade80; color: white; font-size: 24px; cursor: pointer; transition: transform 0.1s;" onmousedown="this.style.transform='scale(0.95)'" onmouseup="this.style.transform='scale(1)'">+</button>
        </div>
        <script>
          let count = 0;
          function increment() { count++; update(); }
          function decrement() { count--; update(); }
          function update() { document.getElementById('count').textContent = count; }
        </script>
      </div>
    HTML
  }
}

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
  puts "\n  sqlite3 #{DB_PATH} \"INSERT INTO webhooks (id, name, channel_id, token, created_by) VALUES ('widget-webhook', 'Widget Webhook', '#{channel['id']}', 'widget-token', 'system');\""
  exit 1
end

puts "=== HTML Widget Webhook Demo ==="
puts "\nAvailable webhooks:"
webhooks.each_with_index do |wh, i|
  channel = db.execute("SELECT name FROM channels WHERE id = ?", wh['channel_id']).first
  puts "  #{i + 1}. #{wh['name']} -> ##{channel['name']}"
end

print "\nSelect webhook number: "
webhook = webhooks[gets.chomp.to_i - 1]

puts "\nAvailable widgets:"
WIDGETS.each_with_index do |(name, _), i|
  puts "  #{i + 1}. #{name}"
end
puts "  #{WIDGETS.size + 1}. Send all widgets"

print "\nSelect widget number: "
choice = gets.chomp.to_i

url = "#{BASE_URL}/api/webhooks/incoming/#{webhook['id']}/#{webhook['token']}"

def send_widget(url, widget)
  response = HTTParty.post(
    url,
    headers: { 'Content-Type' => 'application/json' },
    body: {
      content: widget[:content],
      html: widget[:html],
      widget_size: widget[:widget_size] || 'medium',
      username: widget[:username]
    }.to_json
  )

  if response.code == 200
    puts "  ✓ Sent successfully (size: #{widget[:widget_size] || 'medium'})"
  else
    puts "  ✗ Failed: #{response.body}"
  end
end

if choice == WIDGETS.size + 1
  # Send all widgets
  WIDGETS.each do |name, widget|
    sleep 2
    puts "\nSending #{name} widget..."
    send_widget(url, widget)
    sleep 1
  end
else
  # Send selected widget
  name, widget = WIDGETS.to_a[choice - 1]
  puts "\nSending #{name} widget to #{url}..."
  send_widget(url, widget)
end

puts "\nDone!"
