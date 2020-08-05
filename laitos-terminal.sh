#!/usr/bin/env bash

set -euo pipefail

for prog in curl dialog nslookup socat nslookup; do
  if ! command -v "$prog" &>/dev/null; then
    echo "laitos terminal depends on program $prog, please install it on the computer." >&2
    exit 1
  fi
done

# Application constants
snmp_oid_laitos_ip=1.3.6.1.4.1.52535.121.100
caption_online='✅Online'
caption_offline='➖Unreachable'
caption_unknown='❔Undetermined'
conf_file="$(readlink -f ~/.laitos-terminal-config.txt)"
self_exe="$0"

# Application configuration
set -a
# shellcheck source=/dev/null
if [ -r "$conf_file" ]; then
  source "$conf_file"
fi
set +a
laitos_host="${laitos_host:-}"

if [ "$laitos_host" ]; then
  # Ensure that all configuration variables are defined even though some may be empty
  port_plainsocket="${port_plainsocket:-}"
  port_dnsd="${port_dnsd:-}"
  port_smtpd="${port_smtpd:-}"
  port_snmpd="${port_snmpd:-}"
  port_httpd="${port_httpd:-}"
  port_httpsd="${port_httpsd:-}"
  port_qotd="${port_qotd:-}"
  snmp_community_string="${snmp_community_string:-}"
  app_cmd_pass="${app_cmd_pass:-}"
  app_cmd_endpoint="${app_cmd_endpoint:-}"
  low_bandwidth_mode="${low_bandwidth_mode:-}"
else
  # User has not configured the terminal yet, use the default value to give them a good hint of how the values look.
  port_plainsocket="${port_plainsocket:-23}"
  port_dnsd="${port_dnsd:-53}"
  port_smtpd="${port_smtpd:-25}"
  port_snmpd="${port_snmpd:-161}"
  port_httpd="${port_httpd:-80}"
  port_httpsd="${port_httpsd:-443}"
  port_qotd="${port_qotd:-17}"
  snmp_community_string="${snmp_community_string:-}"
  app_cmd_pass="${app_cmd_pass:-PasswordPIN}"
  app_cmd_endpoint="${app_cmd_endpoint:-/very-secret-app-command-endpoint}"
  low_bandwidth_mode="${low_bandwidth_mode:-n}"
fi

probe_timeout_sec=3
if [ "$low_bandwidth_mode" == 'y' ]; then
  # Low bandwidth mode (e.g. satellite Internet, phone modem) comes with significantly increased latency as well
  probe_timeout_sec=10
fi

# Runtime data
connection_report_file="$(mktemp -p /tmp laitos-terminal-connection-report-XXXXX)"
last_reqresp_file="$(mktemp -p /tmp laitos-terminal-last-reqresp-XXXXX)"
app_cmd_out_file="$(mktemp -p /tmp laitos-terminal-app-cmd-output-XXXXX)"
form_submission_file="$(mktemp -p /tmp laitos-terminal-form-submission-XXXXX)"

# Clean up after temporary files and background jobs on exit
function clean_up_before_exit {
  rm -f "$connection_report_file" "$last_reqresp_file" "$app_cmd_out_file" "$form_submission_file" || true
  readarray -t bg_jobs < <(jobs -p)
  if [ "${#bg_jobs}" -gt 0 ]; then
    kill "${bg_jobs[@]}" &>/dev/null || true
  fi
}
function on_exit {
  clean_up_before_exit
  exit 0
}
trap on_exit EXIT INT TERM

################################################################################
# Communicate with laitos server
################################################################################
function invoke_app_command {
  # cmd is the laitos app command without leading password PIN
  local cmd
  cmd="$1"
  echo -e "REQ: $cmd\nRESP: " > "$last_reqresp_file"
  local endpoint
  endpoint="https://$laitos_host:$port_httpd""$app_cmd_endpoint"
  if [ "$low_bandwidth_mode" ] && [ "$port_httpd" ] || [ ! "$port_httpsd" ]; then
    # In low bandwidth mode, sacrifice security for much shorter round trip by avoiding TLS handshake.
    endpoint="http://$laitos_host:$port_httpd""$app_cmd_endpoint"
  fi
  curl --no-progress-meter -X POST --max-time 60 "$endpoint" -F "cmd=$app_cmd_pass""$cmd" &> "$app_cmd_out_file" || true
  cat "$app_cmd_out_file" >> "$last_reqresp_file"
}

################################################################################
# Background connection status reporting
################################################################################
declare -a daemon_names
daemon_names+=('HTTP server')
daemon_names+=('HTTPS server')
daemon_names+=('DNS server')
daemon_names+=('Mail server')
daemon_names+=('Telnet server')
daemon_names+=('SNMP server')
daemon_names+=('QOTD')

declare -A daemon_connection_status
for daemon_name in "${daemon_names[@]}"; do
  daemon_connection_status["$daemon_name"]="$caption_unknown"
done

function loop_get_latest_reqresp {
  truncate -s 0 "$last_reqresp_file"
  printf 'Use the App Menu to get started' >> "$last_reqresp_file"
}

loop_get_latest_reqresp &

function write_conn_status_file {
  truncate -s 0 "$connection_report_file"
  for daemon_name in "${daemon_names[@]}"; do
    printf '%-20s %s\n' "$daemon_name" "${daemon_connection_status[$daemon_name]}" >> "$connection_report_file"
  done
  echo -en "\nTested at: $(date --rfc-3339=seconds)" >> "$connection_report_file"
}

function loop_get_latest_conn_status {
  while true; do
    if [ "$laitos_host" ]; then
      timeout "$probe_timeout_sec" socat /dev/null "TCP:$laitos_host:$port_httpd" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['HTTP server']=$port_status
      write_conn_status_file

      timeout "$probe_timeout_sec" socat /dev/null "TCP:$laitos_host:$port_httpsd" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['HTTPS server']=$port_status
      write_conn_status_file

      timeout "$probe_timeout_sec" nslookup "-port=$port_dnsd" 'github.com' "$laitos_host" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['DNS server']=$port_status
      write_conn_status_file

      timeout "$probe_timeout_sec" socat /dev/null "TCP:$laitos_host:$port_smtpd" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['Mail server']=$port_status
      write_conn_status_file

      timeout "$probe_timeout_sec" socat /dev/null "TCP:$laitos_host:$port_plainsocket" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['Telnet server']=$port_status
      write_conn_status_file

      echo "$probe_timeout_sec" snmpwalk -v 2c -c "$snmp_community_string" "$laitos_host:$port_snmpd" "$snmp_oid_laitos_ip" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['SNMP server']=$port_status
      write_conn_status_file

      timeout "$probe_timeout_sec" socat /dev/null "TCP:$laitos_host:$port_qotd" &>/dev/null && port_status=$caption_online || port_status=$caption_offline
      daemon_connection_status['QOTD']=$port_status
      write_conn_status_file
    else
      echo 'Please visit menu "Configure laitos"' > "$connection_report_file"
    fi
    sleep "$probe_timeout_sec"
  done
}

loop_get_latest_conn_status &

################################################################################
# Dialog - application configuration
################################################################################
function dialog_config {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '💾 Configure laitos server address and more' --mixedform "The settings for connecting to your laitos server are saved to $conf_file" 21 70 12 \
      'What is the server hostname/IP?' 1 0 "$laitos_host"     1 32 200 0 0 \
      '' 2 0 '' 2 0 0 0 0 \
      'On which port does the server run... (leave empty if unsed)' 3 0 '' 3 0 0 0 0 \
      'HTTP port'                      4 0 "$port_httpd"       4 32 8 0 0 \
      'HTTPS port'                     5 0 "$port_httpsd"      5 32 8 0 0 \
      'DNS port'                       6 0 "$port_dnsd"        6 32 8 0 0 \
      'SMTP port'                      7 0 "$port_smtpd"       7 32 8 0 0 \
      'SNMP port'                      8 0 "$port_snmpd"       8 32 8 0 0 \
      'Telnet - plain socket port'     9 0 "$port_plainsocket" 9 32 8 0 0 \
      'Simple IP service - QOTD port' 10 0 "$port_qotd"       10 32 8 0 0 \
      '' 11 0 '' 11 0 0 0 0 \
      'In order to execute app commands on your laitos server...' 12 0 '' 12 0 0 0 0 \
      'Command processor password'    13 0 "$app_cmd_pass"     13 32 200 0 0 \
      'App command execution API URL' 14 0 "$app_cmd_endpoint" 14 32 200 0 0 \
      '' 15 0 '' 15 0 0 0 0 \
      'What is the community string for probing SNMP?' 16 0 '' 16 0 0 0 0 \
      'Leave empty if unused'       17 0 "$snmp_community_string" 17 32 200 0 0 \
      '' 18 0 '' 18 0 0 0 0 \
      'Low bandwidth mode works better over satellite (reduce security)' 19 0 '' 19 0 0 0 0 \
      'Use low bandwidth mode? (y/n)' 20 0 "$low_bandwidth_mode" 20 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  if [ -s "$form_submission_file" ]; then
    readarray -t form_fields < "$form_submission_file"
    cat << EOF > "$conf_file"
laitos_host="${form_fields[0]}"
port_httpd="${form_fields[1]}"
port_httpsd="${form_fields[2]}"
port_dnsd="${form_fields[3]}"
port_smtpd="${form_fields[4]}"
port_snmpd="${form_fields[5]}"
port_plainsocket="${form_fields[6]}"
port_qotd="${form_fields[7]}"
app_cmd_pass="${form_fields[8]}"
app_cmd_endpoint="${form_fields[9]}"
snmp_community_string="${form_fields[10]}"
low_bandwidth_mode="${form_fields[11]}"
EOF
    # Re-execute this terminal program for background functions to pick up new configuration
    clean_up_before_exit
    exec "$self_exe"
  fi
}

################################################################################
# Dialog - app commands and related
################################################################################
function dialog_app_command_in_progress {
  local bandwidth_mode
  bandwidth_mode=''
  if [ "$low_bandwidth_mode" ]; then
    bandwidth_mode='(low bandwidth mode)'
  fi
  dialog \
    --sleep 1 \
    --backtitle 'Laitos Terminal' \
    --begin 2 50 --title "Running app command $bandwidth_mode" --infobox "Please wait, this may take couple of seconds." 10 45 || true
}

function dialog_app_command_done {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title 'Command result (scroll with Left/Right/Up/Down)' --textbox "$app_cmd_out_file" 21 70 || true
}

function dialog_simple_info_box {
  local info_box_txt
  info_box_txt="$1"
  dialog \
    --sleep 3 \
    --backtitle 'Laitos Terminal' \
    --begin 2 50 --title 'Notice' --infobox "$info_box_txt" 10 45 || true
}

################################################################################
# Dialog - read and send Emails
################################################################################
function dialog_email {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '📮 Read and send Emails' --mixedform '' 21 70 14 \
      'List Emails'                       1 0 ''     1  0   0 0 0 \
      'Email account nick name'           2 0 ''     2 32 200 0 0 \
      'Skip latest N Emails'              3 0 '0'    3 32 200 0 0 \
      'And then list N Emails'            4 0 '10'   4 32 200 0 0 \
      '------------------------------'    5 0 ''     5 0    0 0 0 \
      'Read Email'                        6 0 ''     6  0   0 0 0 \
      'Email account nick name'           7 0 ''     7 32 200 0 0 \
      'Email message number'              8 0 ''     8 32 200 0 0 \
      '------------------------------'    9 0 ''     9  0   0 0 0 \
      'Send Email'                       10 0 ''    10  0   0 0 0 \
      'To address'                       11 0 ''    11 32 200 0 0 \
      'Subject'                          12 0 ''    12 32 200 0 0 \
      'Content'                          13 0 ''    13 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  readarray -t form_fields < "$form_submission_file"
  if [ -s "$form_submission_file" ]; then
    list_acct_nick="${form_fields[0]}"
    list_skip_count="${form_fields[1]}"
    list_get_count="${form_fields[2]}"

    read_acct_nick="${form_fields[3]}"
    read_num="${form_fields[4]}"

    send_to_addr="${form_fields[5]}"
    send_subject="${form_fields[6]}"
    send_content="${form_fields[7]}"
    # Figure out which function user would like to use
    if [ "$list_acct_nick" ]; then
      invoke_app_command ".il $list_acct_nick $list_skip_count $list_get_count" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$read_acct_nick" ]; then
      invoke_app_command ".ir $read_acct_nick $read_num" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$send_to_addr" ]; then
      invoke_app_command ".m $send_to_addr \"$send_subject\" $send_content"
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    else
      dialog_simple_info_box "Please complete any section of the form to use that app."
      dialog_email
    fi
  fi
}

################################################################################
# Dialog - make calls and send SMS
################################################################################
function dialog_phone {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '📠 Make calls and send SMS' --mixedform '' 21 70 14 \
      'Dial a number and speak a message' 1 0 ''     1  0   0 0 0 \
      'Dial phone number (+35812345)'     2 0 ''     2 32 200 0 0 \
      'Speak message'                     3 0 ''     3 32 200 0 0 \
      '------------------------------'    4 0 ''     4 0    0 0 0 \
      'Send an SMS'                       5 0 ''     5  0   0 0 0 \
      'To number (+35812345)'             6 0 ''     6 32 200 0 0 \
      'Text message'                      7 0 ''     7 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  readarray -t form_fields < "$form_submission_file"
  if [ -s "$form_submission_file" ]; then
    dial_number="${form_fields[0]}"
    speak_message="${form_fields[1]}"

    send_to_number="${form_fields[2]}"
    text_message="${form_fields[3]}"
    # Figure out which function user would like to use
    if [ "$dial_number" ]; then
      invoke_app_command ".pc $dial_number $speak_message" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$send_to_number" ]; then
      invoke_app_command ".pt $send_to_number $text_message" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    else
      dialog_simple_info_box "Please complete any section of the form to use that app."
      dialog_phone
    fi
  fi
}

################################################################################
# Dialog - read and post tweets
################################################################################
function dialog_tweet {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '🐦 Read and post tweets' --mixedform '' 21 70 14 \
      'Read latest tweets from home timeline' 1 0 ''     1  0   0 0 0 \
      'Skip latest N tweets'                  2 0 '0'    2 32 200 0 0 \
      'And then read N tweets'                3 0 '10'   3 32 200 0 0 \
      '------------------------------'        4 0 ''     4 0    0 0 0 \
      'Post a tweet'                          5 0 ''     5  0   0 0 0 \
      'Content'                               6 0 ''     6 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  readarray -t form_fields < "$form_submission_file"
  if [ -s "$form_submission_file" ]; then
    skip_n_tweets="${form_fields[0]}"
    read_n_tweets="${form_fields[1]}"

    tweet_content="${form_fields[2]}"
    # Figure out which function user would like to use
    # The read tweet fields use default values, hence check posting of new tweet first.
    if [ "$tweet_content" ]; then
      invoke_app_command ".tp $tweet_content" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$read_n_tweets" ]; then
      invoke_app_command ".tg $skip_n_tweets $read_n_tweets" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    else
      dialog_simple_info_box "Please complete any section of the form to use that app."
      dialog_tweet
    fi
  fi
}

################################################################################
# Dialog - get the latest news / weather / facts
################################################################################
function dialog_info {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '🌐 Get the latest news / weather / facts' --mixedform '' 21 70 14 \
      'Get the latest news from RSS'        1 0 ''     1  0   0 0 0 \
      'Skip latest N news articles'         2 0 '0'    2 32 200 0 0 \
      'And then read N articles'            3 0 '10'   3 32 200 0 0 \
      '------------------------------'      4 0 ''     4 0    0 0 0 \
      'Ask WolframAlpha for weather/facts'  5 0 ''     5  0   0 0 0 \
      'Inquiry (free form text)'            6 0 ''     6 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  readarray -t form_fields < "$form_submission_file"
  if [ -s "$form_submission_file" ]; then
    skip_n_feeds="${form_fields[0]}"
    read_n_feeds="${form_fields[1]}"

    wolframalpha_query="${form_fields[2]}"
    # Figure out which function user would like to use
    # The read news fields use default values, hence check WolframApha inquiry first.
    if [ "$wolframalpha_query" ]; then
      invoke_app_command ".w $wolframalpha_query" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$read_n_feeds" ]; then
      invoke_app_command ".r $skip_n_feeds $read_n_feeds" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    else
      dialog_simple_info_box "Please complete any section of the form to use that app."
      dialog_info
    fi
  fi
}

################################################################################
# Dialog - 2FA code / password book / text search
################################################################################
function dialog_book {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '📝 2FA code / password book / text search' --mixedform '' 21 70 14 \
      'Get 2FA authentication code'         1 0 ''     1  0   0 0 0 \
      'The remaining decryption key'        2 0 ''     2 32 200 0 0 \
      'Search for account'                  3 0 ''     3 32 200 0 0 \
      '------------------------------'      4 0 ''     4  0   0 0 0 \
      'Find in encrypted text'              5 0 ''     5  0   0 0 0 \
      'File shortcut word'                  6 0 ''     6 32 200 0 0 \
      'The remaining decryption key'        7 0 ''     7 32 200 0 0 \
      'Search for'                          8 0 ''     8 32 200 0 0 \
      '------------------------------'      9 0 ''     9  0   0 0 0 \
      'Find in plain text'                 10 0 ''    10  0   0 0 0 \
      'File shortcut word'                 11 0 ''    11 32 200 0 0 \
      'Search for'                         12 0 ''    12 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  readarray -t form_fields < "$form_submission_file"
  if [ -s "$form_submission_file" ]; then
    twofa_decrypt_key="${form_fields[0]}"
    twofa_search="${form_fields[1]}"

    enc_shortcut="${form_fields[2]}"
    enc_decrypt_key="${form_fields[3]}"
    enc_search="${form_fields[4]}"

    plain_shortcut="${form_fields[5]}"
    plain_search="${form_fields[6]}"
    # Figure out which function user would like to use
    if [ "$twofa_search" ]; then
      invoke_app_command ".2 $twofa_decrypt_key $twofa_search" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$enc_search" ]; then
      invoke_app_command ".a $enc_shortcut $enc_decrypt_key $enc_search" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$plain_search" ]; then
      invoke_app_command ".g $plain_shortcut $plain_search" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    else
      dialog_simple_info_box "Please complete any section of the form to use that app."
      dialog_book
    fi
  fi
}

################################################################################
# Dialog - run commands and inspect server status
################################################################################
function dialog_cmd {
  dialog \
    --backtitle 'Laitos Terminal' \
    --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
    --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
    --and-widget --keep-window --begin 2 50 --title '💻 Run commands and inspect server status' --mixedform '' 21 70 14 \
      'Select one of the following by entering Y' 1 0 ''     1  0   0 0 0 \
      'Get the latest server info'                2 0 'y'    2 32 200 0 0 \
      'Get the latest server log'                 3 0 ''     3 32 200 0 0 \
      'Get the latest server warnings'            4 0 ''     4 32 200 0 0 \
      'Server emergency lock (careful)'           5 0 ''     5 32 200 0 0 \
      '------------------------------'            6 0 ''     6 0    0 0 0 \
      'Run this app command'                      7 0 ''     7 32 200 0 0 \
      \
  2>"$form_submission_file" || return 0
  readarray -t form_fields < "$form_submission_file"
  if [ -s "$form_submission_file" ]; then
    select_get_info="${form_fields[0]}"
    select_get_log="${form_fields[1]}"
    select_get_warn="${form_fields[2]}"
    select_emer_lock="${form_fields[3]}"

    run_app_cmd="${form_fields[4]}"
    # Figure out which function user would like to use
    # The "get latest info" field uses default value, hence check app command input first.
    if [ "$run_app_cmd" ]; then
      invoke_app_command "$run_app_cmd" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$select_get_info" ]; then
      invoke_app_command ".einfo" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$select_get_log" ]; then
      invoke_app_command ".elog" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$select_get_warn" ]; then
      invoke_app_command ".ewarn" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    elif [ "$select_emer_lock" ]; then
      invoke_app_command ".elock" &
      bg_pid=$!
      while kill -0 "$bg_pid"; do
        dialog_app_command_in_progress
      done
      dialog_app_command_done
    else
      dialog_simple_info_box "Please complete any section of the form to use that app."
      dialog_phone
    fi
  fi
}
################################################################################
# Main menu
################################################################################
declare -A main_menu_key_labels
main_menu_key_labels['config']='💾 Configure laitos server address and more'
main_menu_key_labels['email']='📮 Read and send Emails'
main_menu_key_labels['phone']='📠 Make calls and send SMS'
main_menu_key_labels['tweet']='🐦 Read and post tweets'
main_menu_key_labels['info']='🌐 Get the latest news / weather / facts'
main_menu_key_labels['book']='📝 2FA code / password book / text search'
main_menu_key_labels['cmd']='💻 Run commands and inspect server status'

function dialog_main_menu {
  while true; do
    exec 5>&1
    main_menu_choice=$(
    dialog \
      --backtitle 'Laitos Terminal' \
      --keep-window --begin 2 2 --title "Connection - $laitos_host" --tailboxbg "$connection_report_file" 12 45 \
      --and-widget --begin 16 2 --title 'Last contact' --tailboxbg "$last_reqresp_file" 7 45 \
      --and-widget --keep-window --begin 2 50 --title "App Menu" --radiolist "Welcome to laitos terminal! What can I do for you?" 21 70 10 \
        "${main_menu_key_labels['config']}" '' 'ON' \
        "${main_menu_key_labels['email']}" '' '' \
        "${main_menu_key_labels['phone']}" '' '' \
        "${main_menu_key_labels['tweet']}" '' '' \
        "${main_menu_key_labels['info']}" '' '' \
        "${main_menu_key_labels['book']}" '' '' \
        "${main_menu_key_labels['cmd']}" '' '' \
    2>&1 1>&5 || true
    )
    exec 5>&-
    if [ ! "$main_menu_choice" ]; then
      echo 'Thanks for using laitos terminal, see you next time!'
      exit 0
    fi

    case "$main_menu_choice" in
      "${main_menu_key_labels['config']}")
        dialog_config
        ;;
      "${main_menu_key_labels['email']}")
        dialog_email
        ;;
      "${main_menu_key_labels['phone']}")
        dialog_phone
        ;;
      "${main_menu_key_labels['tweet']}")
        dialog_tweet
        ;;
      "${main_menu_key_labels['info']}")
        dialog_info
        ;;
      "${main_menu_key_labels['book']}")
        dialog_book
        ;;
      "${main_menu_key_labels['cmd']}")
        dialog_cmd
        ;;
    esac
  done
}

dialog_main_menu
