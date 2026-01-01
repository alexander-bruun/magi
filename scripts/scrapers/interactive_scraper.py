#!/usr/bin/env python3

import os
import sys
import subprocess
import json
import signal
import threading
from pathlib import Path
import curses
import re

# Global log storage for parallel scraping
scraper_logs = {}
log_lock = threading.Lock()
scraper_status = {}  # 'running', 'completed', 'failed'

def strip_ansi_codes(text):
    """Strip ANSI escape sequences from text."""
    ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
    return ansi_escape.sub('', text)

def parse_ansi_line(line):
    """Parse ANSI colored line into segments with curses attributes."""
    # Simple parser for basic ANSI codes
    segments = []
    current_text = ""
    current_fg = -1  # default
    current_bg = -1
    current_bold = False
    
    i = 0
    while i < len(line):
        if line[i] == '\x1b' and i + 1 < len(line) and line[i+1] == '[':
            # Start of ANSI sequence
            if current_text:
                segments.append((current_text, current_fg, current_bg, current_bold))
                current_text = ""
            
            # Find end of sequence
            j = i + 2
            while j < len(line) and line[j] not in 'm':
                j += 1
            if j < len(line):
                codes = line[i+2:j].split(';')
                for code in codes:
                    if code == '0':
                        current_fg = -1
                        current_bg = -1
                        current_bold = False
                    elif code == '1':
                        current_bold = True
                    elif 30 <= int(code) <= 37:
                        current_fg = int(code) - 30
                    elif 40 <= int(code) <= 47:
                        current_bg = int(code) - 40
                i = j + 1
            else:
                i += 1
        else:
            current_text += line[i]
            i += 1
    
    if current_text:
        segments.append((current_text, current_fg, current_bg, current_bold))
    
    return segments

def ansi_to_curses_attr(fg, bg, bold, has_colors):
    """Convert ANSI colors to curses attribute."""
    if not has_colors:
        return 0
    
    attr = 0
    if bold:
        attr |= curses.A_BOLD
    
    # Map ANSI fg colors to curses color pairs (10-17)
    if fg >= 0 and fg <= 7:
        pair_num = 10 + fg
        attr |= curses.color_pair(pair_num)
    
    # For now, ignore bg as it's rare in logs
    
    return attr

# Available scrapers
SCRAPERS = {
    '1': {'name': 'Asura Scans', 'file': 'asurascans.py', 'default_folder': 'AsuraScans'},
    '2': {'name': 'Demonic Scans', 'file': 'demonicscans.py', 'default_folder': 'DemonicScans'},
    '3': {'name': 'Flame Comics', 'file': 'flamecomics.py', 'default_folder': 'FlameComics'},
    '4': {'name': 'HiveToons', 'file': 'hivetoons.py', 'default_folder': 'HiveToons'},
    '5': {'name': 'LHTranslation', 'file': 'lhtranslation.py', 'default_folder': 'LHTranslation'},
    '6': {'name': 'MangaKatana', 'file': 'mangakatana.py', 'default_folder': 'MangaKatana'},
    '7': {'name': 'Omega Scans', 'file': 'omegascans.py', 'default_folder': 'OmegaScans'},
    '8': {'name': 'Qi Scans', 'file': 'qiscans.py', 'default_folder': 'QiScans'},
    '9': {'name': 'Reset Scans', 'file': 'resetscans.py', 'default_folder': 'ResetScans'},
    '10': {'name': 'Thunder Scans', 'file': 'thunderscans.py', 'default_folder': 'ThunderScans'},
    '11': {'name': 'UToon', 'file': 'utoon.py', 'default_folder': 'UToon'},
    '12': {'name': 'Vortex Scans', 'file': 'vortexscans.py', 'default_folder': 'VortexScans'},
    '13': {'name': 'Z Scans', 'file': 'zscans.py', 'default_folder': 'ZScans'},
}

def load_config():
    """Load configuration from config.json file."""
    config_path = Path(__file__).parent / 'config.json'
    if not config_path.exists():
        return {}

    try:
        with open(config_path, 'r') as f:
            return json.load(f)
    except Exception as e:
        print(f"Warning: Could not load config file: {e}")
        return {}

def get_scraper_config(config, scraper_key):
    """Get scraper-specific configuration."""
    scraper_file = SCRAPERS[scraper_key]['file'].replace('.py', '')
    return config.get('scrapers', {}).get(scraper_file, {})

def clear_screen():
    """Clear the terminal screen."""
    os.system('clear' if os.name != 'nt' else 'cls')

def print_header():
    """Print the application header."""
    print("=" * 60)
    print("           MAGI SCRAPER INTERACTIVE MENU")
    print("=" * 60)
    print()

def print_scraper_menu():
    """Print the scraper selection menu."""
    print("Available Scrapers:")
    print("-" * 40)
    for key, scraper in SCRAPERS.items():
        print(f"{key}. {scraper['name']}")
    print()
    print("13. Run all scrapers in parallel")
    print("0. Exit")
    print()

def get_scraper_choice_curses(stdscr):
    """Get user's scraper choice using curses for arrow key navigation."""
    # Clear screen and set up
    stdscr.clear()
    curses.curs_set(0)  # Hide cursor
    stdscr.keypad(True)  # Enable keypad

    # Initialize colors if supported
    if curses.has_colors():
        curses.start_color()
        curses.init_pair(1, curses.COLOR_WHITE, curses.COLOR_CYAN)  # Highlighted: white on cyan

    # Menu items
    menu_items = []
    for key, scraper in SCRAPERS.items():
        menu_items.append((key, scraper['name']))
    menu_items.append(('13', 'Run all scrapers in parallel'))
    menu_items.append(('0', 'Exit'))

    current_row = 0
    max_row = len(menu_items) - 1

    while True:
        # Print header
        stdscr.addstr(0, 0, "=" * 60)
        stdscr.addstr(1, 0, "           MAGI SCRAPER INTERACTIVE MENU")
        stdscr.addstr(2, 0, "=" * 60)
        stdscr.addstr(4, 0, "Available Scrapers:")
        stdscr.addstr(5, 0, "-" * 40)

        # Print menu items
        for i, (key, name) in enumerate(menu_items):
            if i == current_row:
                if curses.has_colors():
                    stdscr.attron(curses.color_pair(1))  # White on cyan for highlight
                else:
                    stdscr.attron(curses.A_REVERSE)  # Fallback to reverse
                stdscr.addstr(6 + i, 0, f"{key}. {name}")
                if curses.has_colors():
                    stdscr.attroff(curses.color_pair(1))
                else:
                    stdscr.attroff(curses.A_REVERSE)
            else:
                stdscr.addstr(6 + i, 0, f"{key}. {name}")  # Default colors for normal items

        stdscr.addstr(6 + len(menu_items) + 1, 0, "Use arrow keys to navigate, Enter to select, 'q' to quit")

        # Refresh screen
        stdscr.refresh()

        # Get user input
        key = stdscr.getch()

        if key == curses.KEY_UP and current_row > 0:
            current_row -= 1
        elif key == curses.KEY_DOWN and current_row < max_row:
            current_row += 1
        elif key == ord('\n') or key == curses.KEY_ENTER:  # Enter key
            selected_key = menu_items[current_row][0]
            if selected_key == '0':
                return None
            elif selected_key == '13':
                return 'all'
            return selected_key
        elif key == ord('q') or key == 27:  # 'q' or ESC
            return None

def get_scraper_choice_text():
    """Get user's scraper choice using text-based menu."""
    while True:
        clear_screen()
        print_header()
        print_scraper_menu()

        try:
            choice = input("Enter your choice (0-13): ").strip()
            if choice == '0':
                return None
            elif choice == '13':
                return 'all'
            elif choice in SCRAPERS:
                return choice
            else:
                print("Invalid choice. Please enter a number between 0 and 13.")
                input("Press Enter to continue...")
        except KeyboardInterrupt:
            return None
        except EOFError:
            return None

def get_scraper_choice():
    """Get user's scraper choice."""
    import sys
    if not sys.stdin.isatty():
        # Not an interactive terminal, use text menu
        return get_scraper_choice_text()

    try:
        return curses.wrapper(get_scraper_choice_curses)
    except Exception as e:
        # Fallback to text-based menu if curses fails
        print(f"Warning: Interactive menu not available ({e}), using text menu...")
        return get_scraper_choice_text()

def get_folder_location(scraper, config, scraper_config):
    """Get folder location from user or config."""
    default_folder = scraper['default_folder']
    current_dir = os.getcwd()

    # Check if folder is configured for this scraper specifically
    configured_folder = scraper_config.get('folder')
    if configured_folder:
        if not os.path.isabs(configured_folder):
            configured_folder = os.path.join(current_dir, configured_folder)
        folder_path = configured_folder
        print(f"\nConfigured folder: {folder_path}")
    else:
        # Use defaults folder_base + scraper name, or current_dir + scraper name
        folder_base = config.get('defaults', {}).get('folder_base')
        if folder_base:
            if not os.path.isabs(folder_base):
                folder_base = os.path.join(current_dir, folder_base)
            folder_path = os.path.join(folder_base, default_folder)
        else:
            folder_path = os.path.join(current_dir, default_folder)
        print(f"\nDefault folder: {default_folder}")
        print(f"Current directory: {current_dir}")

    # Use the determined folder
    Path(folder_path).mkdir(parents=True, exist_ok=True)
    print(f"Using folder: {folder_path}")
    return folder_path

    # Otherwise, prompt user
    while True:
        try:
            folder_input = input(f"Enter folder path (press Enter for default '{default_folder}'): ").strip()

            if not folder_input:
                folder_path = os.path.join(current_dir, 'scripts', 'scrapers', default_folder)
            else:
                if os.path.isabs(folder_input):
                    folder_path = folder_input
                else:
                    folder_path = os.path.join(current_dir, folder_input)

            # Create folder if it doesn't exist
            Path(folder_path).mkdir(parents=True, exist_ok=True)

            print(f"Using folder: {folder_path}")
            return folder_path

        except KeyboardInterrupt:
            print("\nOperation cancelled.")
            return None
        except Exception as e:
            print(f"Error with folder path: {e}")
            continue

def get_additional_options(config, scraper_config, parallel=False):
    """Get additional scraper options from config or user."""
    print("\nAdditional Options:")
    print("-" * 20)

    options = {}

    # Dry run - check config first
    dry_run_config = scraper_config.get('dry_run')
    if dry_run_config is not None:
        options['dry_run'] = 'true' if dry_run_config else 'false'
        print(f"Dry Run: {'Enabled' if dry_run_config else 'Disabled'} (from config)")
    else:
        default_dry_run = config.get('defaults', {}).get('dry_run', False)
        options['dry_run'] = 'true' if default_dry_run else 'false'
        print(f"Dry Run: {'Enabled' if default_dry_run else 'Disabled'} (default)")

    # Convert to WebP - check config first
    convert_webp_config = scraper_config.get('convert_to_webp')
    if convert_webp_config is not None:
        options['convert_to_webp'] = 'true' if convert_webp_config else 'false'
        print(f"Convert to WebP: {'Enabled' if convert_webp_config else 'Disabled'} (from config)")
    else:
        default_convert_webp = config.get('defaults', {}).get('convert_to_webp', True)
        options['convert_to_webp'] = 'true' if default_convert_webp else 'false'
        print(f"Convert to WebP: {'Enabled' if default_convert_webp else 'Disabled'} (default)")

    # If all options are from config or parallel mode, skip prompting
    if (dry_run_config is not None and convert_webp_config is not None) or parallel:
        return options

    print("\nModify options? (press Enter to keep current values)")

    # Dry run
    if dry_run_config is None:
        while True:
            try:
                dry_run_input = input(f"Enable dry run mode? ({'y' if default_dry_run else 'n'}/N): ").strip().lower()
                if dry_run_input in ['y', 'yes']:
                    options['dry_run'] = 'true'
                    break
                elif dry_run_input in ['', 'n', 'no']:
                    options['dry_run'] = 'false'
                    break
                else:
                    print("Please enter 'y' for yes or 'n' for no.")
            except KeyboardInterrupt:
                print("\nOperation cancelled.")
                return None

    # Convert to WebP
    if convert_webp_config is None:
        while True:
            try:
                convert_webp_input = input(f"Convert to WebP? ({'y' if default_convert_webp else 'n'}/Y): ").strip().lower()
                if convert_webp_input in ['', 'y', 'yes']:
                    options['convert_to_webp'] = 'true'
                    break
                elif convert_webp_input in ['n', 'no']:
                    options['convert_to_webp'] = 'false'
                    break
                else:
                    print("Please enter 'y' for yes or 'n' for no.")
            except KeyboardInterrupt:
                print("\nOperation cancelled.")
                return None

    return options

def confirm_and_run(scraper, folder_path, options, config, scraper_config, parallel=False):
    """Confirm settings and run the scraper."""
    scraper_name = scraper['name']
    print("\n" + "=" * 60)
    print("CONFIRMATION")
    print("=" * 60)
    print(f"Scraper: {scraper['name']}")
    print(f"Folder: {folder_path}")
    print(f"Dry Run: {options['dry_run']}")
    print(f"Convert to WebP: {options['convert_to_webp']}")
    print()

    # Check if auto-confirm is enabled
    auto_confirm = scraper_config.get('auto_confirm', False) or config.get('defaults', {}).get('auto_confirm', False)
    if auto_confirm or parallel:
        print("Auto-confirm enabled, starting scraper...")
    else:
        while True:
            try:
                confirm = input("Start scraping? (y/N): ").strip().lower()
                if confirm in ['y', 'yes']:
                    break
                elif confirm in ['', 'n', 'no']:
                    print("Operation cancelled.")
                    return
                else:
                    print("Please enter 'y' for yes or 'n' for no.")
            except KeyboardInterrupt:
                print("\nOperation cancelled.")
                return

    # Prepare environment
    env = os.environ.copy()
    env['folder'] = folder_path
    env['dry_run'] = options['dry_run']
    env['convert_to_webp'] = options['convert_to_webp']

    # Run the scraper
    scraper_path = os.path.join(os.path.dirname(__file__), scraper['file'])

    if not os.path.exists(scraper_path):
        print(f"Error: Scraper file '{scraper_path}' not found!")
        return

    print(f"\nStarting {scraper['name']} scraper...")
    print("-" * 50)

    # Prepare command
    python_exe = sys.executable
    module_name = scraper['file'].replace('.py', '').replace('-', '_')
    env_json = json.dumps(env)
    script = 'import os, json\nos.environ.update(json.loads(' + repr(env_json) + '))\nimport sys\nsys.path.insert(0, ".")\nimport ' + module_name + '\n' + module_name + '.main()'
    command = [python_exe, '-c', script]

    try:
        if parallel:
            with log_lock:
                scraper_logs[scraper_name].append(f"Folder: {folder_path}")
                scraper_logs[scraper_name].append(f"Dry run: {options['dry_run']}")
                scraper_logs[scraper_name].append(f"Convert to WebP: {options['convert_to_webp']}")
                scraper_logs[scraper_name].append(f"Command: {python_exe} -c import {module_name}; {module_name}.main()")
            
            # Capture output for parallel mode
            proc = subprocess.Popen(command,
                                   env={},  # Empty env since we set it in the command
                                   cwd=os.path.dirname(scraper_path),
                                   stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT,
                                   text=True,
                                   bufsize=1,
                                   universal_newlines=True,
                                   preexec_fn=os.setsid)
            
            # Read output line by line and store in logs
            def read_output():
                try:
                    for line in iter(proc.stdout.readline, ''):
                        if line.strip():
                            with log_lock:
                                scraper_logs[scraper_name].append(line.rstrip('\n\r'))
                    proc.stdout.close()
                except Exception as e:
                    with log_lock:
                        scraper_logs[scraper_name].append(f"Error reading output: {e}")
            
            # Start output reader thread
            output_thread = threading.Thread(target=read_output, daemon=True)
            output_thread.start()
            
            # Wait for process to complete
            result = proc.wait()
            
            # Wait a bit for output thread to finish
            output_thread.join(timeout=1)
            
            # Add completion message
            with log_lock:
                if result == 0:
                    scraper_logs[scraper_name].append(f"✓ {scraper_name} completed successfully!")
                else:
                    scraper_logs[scraper_name].append(f"✗ {scraper_name} failed with exit code {result}")
        else:
            # Normal mode - let output go to terminal
            proc = subprocess.Popen(command,
                                   env={},  # Empty env since we set it in the command
                                   cwd=os.path.dirname(scraper_path),
                                   preexec_fn=os.setsid)
            result = proc.wait()
            print("\n" + "-" * 50)
            if result == 0:
                print(f"✓ {scraper_name} completed successfully!")
            else:
                print(f"✗ {scraper_name} failed with exit code {result}")
            
            return result

    except KeyboardInterrupt:
        print(f"\n⚠ {scraper_name} interrupted by user")
        try:
            # Terminate the process group
            os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
            # Wait a bit for graceful shutdown
            proc.wait(timeout=5)
        except:
            # Force kill if needed
            try:
                os.killpg(os.getpgid(proc.pid), signal.SIGKILL)
            except:
                pass
        # Don't re-raise - continue to menu
        return -1

    except Exception as e:
        error_msg = f"\n✗ Error running {scraper_name}: {e}"
        if parallel:
            with log_lock:
                scraper_logs[scraper_name].append(error_msg)
        else:
            print(error_msg)
        return -1

    print("\n" + "=" * 50)
    if not parallel:
        print("Returning to main menu...")
    
        # Pause to let user see results
        try:
            input("Press Enter to continue...")
        except KeyboardInterrupt:
            pass
    
    return 'menu'

def show_log_viewer():
    """Interactive log viewer with bottom bar navigation."""
    print("Attempting to start curses log viewer...")
    try:
        curses.wrapper(log_viewer_curses)
        print("Curses log viewer completed successfully")
    except Exception as e:
        print(f"Log viewer not available: {e}")
        import traceback
        traceback.print_exc()
        print("Falling back to text-based viewer...")
        show_log_viewer_fallback()

def log_viewer_curses(stdscr):
    """Curses-based log viewer with bottom navigation bar."""
    
    def calculate_nav_rows(nav_items, width):
        """Calculate how many rows needed for navigation bar and organize items into rows."""
        rows = []
        current_row = []
        current_width = 0
        available_width = width - 1  # Leave some margin
        
        for item in nav_items:
            item_width = len(item[0]) + 1  # +1 for space between items
            if current_width + item_width > available_width and current_row:
                rows.append(current_row)
                current_row = [item]
                current_width = item_width
            else:
                current_row.append(item)
                current_width += item_width
        
        if current_row:
            rows.append(current_row)
        
        return rows if rows else [[]]
    
    def render_nav_bar(stdscr, nav_rows, start_y, width, has_colors):
        """Render the navigation bar rows."""
        for row_idx, row in enumerate(nav_rows):
            current_pos = 0
            y = start_y + row_idx
            for name, color in row:
                if current_pos + len(name) >= width:
                    break
                if has_colors or color == curses.A_REVERSE:
                    stdscr.attron(color)
                stdscr.addstr(y, current_pos, name)
                if has_colors or color == curses.A_REVERSE:
                    stdscr.attroff(color)
                current_pos += len(name) + 1
    
    # Set up curses
    curses.curs_set(0)  # Hide cursor
    stdscr.keypad(True)  # Enable keypad
    
    # Try to initialize colors, but don't fail if not available
    try:
        curses.use_default_colors()
        curses.start_color()
        curses.init_pair(1, curses.COLOR_WHITE, curses.COLOR_BLUE)  # Selected scraper
        curses.init_pair(2, curses.COLOR_BLACK, curses.COLOR_WHITE)  # Header
        curses.init_pair(3, curses.COLOR_RED, -1)    # Failed scraper
        curses.init_pair(4, curses.COLOR_GREEN, -1)  # Success scraper
        curses.init_pair(5, curses.COLOR_YELLOW, -1) # Running scraper
        # Additional pairs for ANSI colors
        curses.init_pair(10, curses.COLOR_BLACK, -1)   # ANSI black
        curses.init_pair(11, curses.COLOR_RED, -1)     # ANSI red
        curses.init_pair(12, curses.COLOR_GREEN, -1)   # ANSI green
        curses.init_pair(13, curses.COLOR_YELLOW, -1)  # ANSI yellow
        curses.init_pair(14, curses.COLOR_BLUE, -1)    # ANSI blue
        curses.init_pair(15, curses.COLOR_MAGENTA, -1) # ANSI magenta
        curses.init_pair(16, curses.COLOR_CYAN, -1)    # ANSI cyan
        curses.init_pair(17, curses.COLOR_WHITE, -1)   # ANSI white
        has_colors = True
    except:
        has_colors = False
    
    # Get terminal size
    height, width = stdscr.getmaxyx()
    
    # Get scraper list
    with log_lock:
        scraper_names = list(scraper_logs.keys())
    
    if not scraper_names:
        stdscr.addstr(0, 0, "No logs available")
        stdscr.getch()
        return
    
    current_scraper_idx = 0
    scroll_offset = 0
    
    while True:
        stdscr.clear()
        height, width = stdscr.getmaxyx()  # Get current terminal size
        
        # Get current scraper's logs
        current_scraper = scraper_names[current_scraper_idx]
        with log_lock:
            logs = scraper_logs[current_scraper].copy()
        
        # Create navigation bar items first to calculate rows needed
        nav_items = []
        for i, name in enumerate(scraper_names):
            # Get status from scraper_status or log content
            with log_lock:
                stat = scraper_status.get(name)
                if stat == 'completed':
                    status_indicator = "✓"
                    color = curses.color_pair(4) if has_colors else 0
                elif stat == 'failed':
                    status_indicator = "✗"
                    color = curses.color_pair(3) if has_colors else 0
                else:
                    # Fallback to log content check
                    scraper_logs_data = scraper_logs[name]
                    if any("completed successfully" in line for line in scraper_logs_data):
                        status_indicator = "✓"
                        color = curses.color_pair(4) if has_colors else 0
                    elif any("failed" in line for line in scraper_logs_data):
                        status_indicator = "✗"
                        color = curses.color_pair(3) if has_colors else 0
                    else:
                        status_indicator = "~"
                        color = 0
            
            display_name = f"{status_indicator} {name}"
            if i == current_scraper_idx:
                nav_items.append((display_name, curses.A_REVERSE))
            else:
                nav_items.append((display_name, color))
        
        # Calculate navigation bar rows
        nav_rows = calculate_nav_rows(nav_items, width)
        nav_row_count = len(nav_rows)
        
        # Display header
        header = f"SCRAPER LOGS - {current_scraper} ({len(logs)} lines)"
        if has_colors:
            stdscr.attron(curses.A_BOLD)
        stdscr.addstr(0, 0, header[:width-1])
        if has_colors:
            stdscr.attroff(curses.A_BOLD)
        
        # Display logs (last 100 lines, with scrolling)
        log_area_height = height - 2 - nav_row_count  # Header + nav rows + help line
        display_lines = logs[-100:]  # Last 100 lines
        
        # Adjust scroll offset if needed
        max_scroll = max(0, len(display_lines) - log_area_height)
        # Auto-scroll to bottom for new logs
        scroll_offset = max_scroll
        
        # Display log lines
        for i in range(log_area_height):
            line_idx = scroll_offset + i
            if line_idx < len(display_lines):
                line = display_lines[line_idx]
                # Strip ANSI codes for clean display
                line = strip_ansi_codes(line)
                if len(line) >= width:
                    line = line[:width-1]
                stdscr.addstr(1 + i, 0, line)
        
        # Display navigation bar at bottom
        nav_y = height - 1 - nav_row_count
        render_nav_bar(stdscr, nav_rows, nav_y, width, has_colors)
        
        # Display help text
        help_text = "←→: Switch | q: Quit | r: Refresh"
        stdscr.addstr(height - 1, 0, help_text[:width-1])
        
        stdscr.refresh()
        
        # Handle input
        key = stdscr.getch()
        
        if key == curses.KEY_LEFT and current_scraper_idx > 0:
            current_scraper_idx -= 1
            scroll_offset = max_scroll
        elif key == curses.KEY_RIGHT and current_scraper_idx < len(scraper_names) - 1:
            current_scraper_idx += 1
            scroll_offset = max_scroll
        elif key == ord('q') or key == 27:  # q or ESC
            break
        elif key == ord('r'):
            # Refresh - just continue the loop
            pass

def log_viewer_curses_realtime(stdscr, threads):
    """Curses-based real-time log viewer with bottom navigation bar."""
    
    def calculate_nav_rows(nav_items, width):
        """Calculate how many rows needed for navigation bar and organize items into rows."""
        rows = []
        current_row = []
        current_width = 0
        available_width = width - 1  # Leave some margin
        
        for item in nav_items:
            item_width = len(item[0]) + 1  # +1 for space between items
            if current_width + item_width > available_width and current_row:
                rows.append(current_row)
                current_row = [item]
                current_width = item_width
            else:
                current_row.append(item)
                current_width += item_width
        
        if current_row:
            rows.append(current_row)
        
        return rows if rows else [[]]
    
    def render_nav_bar(stdscr, nav_rows, start_y, width, has_colors):
        """Render the navigation bar rows."""
        for row_idx, row in enumerate(nav_rows):
            current_pos = 0
            y = start_y + row_idx
            for name, color in row:
                if current_pos + len(name) >= width:
                    break
                if has_colors or color == curses.A_REVERSE:
                    stdscr.attron(color)
                stdscr.addstr(y, current_pos, name)
                if has_colors or color == curses.A_REVERSE:
                    stdscr.attroff(color)
                current_pos += len(name) + 1
    
    # Set up curses
    curses.curs_set(0)  # Hide cursor
    stdscr.keypad(True)  # Enable keypad
    stdscr.timeout(1000)  # Refresh every 1 second
    
    # Try to initialize colors, but don't fail if not available
    try:
        curses.use_default_colors()
        curses.start_color()
        curses.init_pair(1, curses.COLOR_WHITE, curses.COLOR_BLUE)  # Selected scraper
        curses.init_pair(2, curses.COLOR_BLACK, curses.COLOR_WHITE)  # Header
        curses.init_pair(3, curses.COLOR_RED, -1)    # Failed scraper
        curses.init_pair(4, curses.COLOR_GREEN, -1)  # Success scraper
        curses.init_pair(5, curses.COLOR_YELLOW, -1) # Running scraper
        # Additional pairs for ANSI colors
        curses.init_pair(10, curses.COLOR_BLACK, -1)   # ANSI black
        curses.init_pair(11, curses.COLOR_RED, -1)     # ANSI red
        curses.init_pair(12, curses.COLOR_GREEN, -1)   # ANSI green
        curses.init_pair(13, curses.COLOR_YELLOW, -1)  # ANSI yellow
        curses.init_pair(14, curses.COLOR_BLUE, -1)    # ANSI blue
        curses.init_pair(15, curses.COLOR_MAGENTA, -1) # ANSI magenta
        curses.init_pair(16, curses.COLOR_CYAN, -1)    # ANSI cyan
        curses.init_pair(17, curses.COLOR_WHITE, -1)   # ANSI white
        has_colors = True
    except:
        has_colors = False
    
    # Get terminal size
    height, width = stdscr.getmaxyx()
    
    # Get scraper list
    with log_lock:
        scraper_names = list(scraper_logs.keys())
    
    if not scraper_names:
        stdscr.addstr(0, 0, "No logs available")
        stdscr.getch()
        return
    
    current_scraper_idx = 0
    scroll_offset = 0
    
    while True:
        # Check if all threads are done
        running_threads = [t for t in threads if t.is_alive()]
        all_done = len(running_threads) == 0
        
        stdscr.clear()
        height, width = stdscr.getmaxyx()  # Get current terminal size
        
        # Get current scraper's logs
        current_scraper = scraper_names[current_scraper_idx]
        with log_lock:
            logs = scraper_logs[current_scraper].copy()
            status = scraper_status.get(current_scraper, 'running')
        
        # Create navigation bar items first to calculate rows needed
        nav_items = []
        for i, name in enumerate(scraper_names):
            # Get status
            with log_lock:
                stat = scraper_status.get(name, 'running')
            if stat == 'completed':
                status_indicator = "✓"
                color = curses.color_pair(4) if has_colors else 0
            elif stat == 'failed':
                status_indicator = "✗"
                color = curses.color_pair(3) if has_colors else 0
            else:  # running
                status_indicator = "~"
                color = curses.color_pair(5) if has_colors else 0
            
            display_name = f"{status_indicator} {name}"
            if i == current_scraper_idx:
                nav_items.append((display_name, curses.A_REVERSE))
            else:
                nav_items.append((display_name, color))
        
        # Calculate navigation bar rows
        nav_rows = calculate_nav_rows(nav_items, width)
        nav_row_count = len(nav_rows)
        
        # Display header
        status_text = "RUNNING" if not all_done else "COMPLETED"
        header = f"SCRAPER LOGS - {current_scraper} ({len(logs)} lines) [{status_text}]"
        if has_colors:
            stdscr.attron(curses.A_BOLD)
        stdscr.addstr(0, 0, header[:width-1])
        if has_colors:
            stdscr.attroff(curses.A_BOLD)
        
        # Display logs (last 100 lines, with scrolling)
        log_area_height = height - 2 - nav_row_count  # Header + nav rows + help line
        display_lines = logs[-100:]  # Last 100 lines
        
        # Adjust scroll offset if needed
        max_scroll = max(0, len(display_lines) - log_area_height)
        # Auto-scroll to bottom for new logs
        scroll_offset = max_scroll
        
        # Display log lines
        for i in range(log_area_height):
            line_idx = scroll_offset + i
            if line_idx < len(display_lines):
                line = display_lines[line_idx]
                # Strip ANSI codes for clean display
                line = strip_ansi_codes(line)
                if len(line) >= width:
                    line = line[:width-1]
                stdscr.addstr(1 + i, 0, line)
        
        # Display navigation bar at bottom
        nav_y = height - 1 - nav_row_count
        render_nav_bar(stdscr, nav_rows, nav_y, width, has_colors)
        
        # Display help text
        if all_done:
            help_text = "←→: Switch | q: Quit"
        else:
            help_text = "←→: Switch | q: Quit (wait for completion)"
        stdscr.addstr(height - 1, 0, help_text[:width-1])
        
        stdscr.refresh()
        
        # If all done, wait for user input to quit
        if all_done:
            stdscr.timeout(-1)  # Blocking input
        
        # Handle input
        key = stdscr.getch()
        
        if key == curses.KEY_LEFT and current_scraper_idx > 0:
            current_scraper_idx -= 1
            scroll_offset = max_scroll
        elif key == curses.KEY_RIGHT and current_scraper_idx < len(scraper_names) - 1:
            current_scraper_idx += 1
            scroll_offset = max_scroll
        elif key == ord('q') or key == 27:  # q or ESC
            if all_done:
                break
            # If not done, ignore quit
        elif key == -1:  # Timeout
            # Just refresh
            pass
        else:
            # Unknown key, ignore
            pass
        
        # If all done, break after handling input
        if all_done and (key == ord('q') or key == 27):
            break

def show_log_viewer_fallback():
    """Fallback text-based log viewer."""
    while True:
        clear_screen()
        print("=" * 60)
        print("           SCRAPER LOG VIEWER")
        print("=" * 60)
        print()
        
        # List available scrapers with log counts
        scraper_list = []
        for i, (name, logs) in enumerate(scraper_logs.items(), 1):
            status = "✓" if any("completed successfully" in line for line in logs) else "✗"
            log_count = len(logs)
            print(f"{i}. {status} {name} ({log_count} lines)")
            scraper_list.append(name)
        
        print()
        print("13. View all logs combined")
        print("0. Back to main menu")
        print()
        
        try:
            choice = input("Select scraper to view logs (0-13): ").strip()
            
            if choice == '0':
                break
            elif choice == '13':
                # Show combined logs
                view_combined_logs()
            elif choice.isdigit() and 1 <= int(choice) <= len(scraper_list):
                scraper_name = scraper_list[int(choice) - 1]
                view_scraper_logs(scraper_name)
            else:
                print("Invalid choice. Please enter a number between 0 and 13.")
                input("Press Enter to continue...")
        except KeyboardInterrupt:
            break
        except EOFError:
            break

def view_scraper_logs(scraper_name):
    """View logs for a specific scraper."""
    while True:
        clear_screen()
        print(f"=" * 60)
        print(f"           LOGS: {scraper_name}")
        print("=" * 60)
        print()
        
        with log_lock:
            logs = scraper_logs.get(scraper_name, [])
        
        if not logs:
            print("No logs available for this scraper.")
        else:
            # Display logs with line numbers
            for i, line in enumerate(logs[-50:], len(logs)-49 if len(logs) > 50 else 1):  # Show last 50 lines
                print(f"{i:3d}: {line}")
            
            if len(logs) > 50:
                print(f"\n... ({len(logs) - 50} more lines)")
        
        print()
        print("Commands:")
        print("  'r' - Refresh logs")
        print("  'a' - View all logs")
        print("  'b' - Back to scraper list")
        print()
        
        try:
            cmd = input("Command: ").strip().lower()
            if cmd == 'b':
                break
            elif cmd == 'r':
                continue  # Refresh by re-displaying
            elif cmd == 'a':
                # Show all logs
                clear_screen()
                print(f"=" * 60)
                print(f"           ALL LOGS: {scraper_name}")
                print("=" * 60)
                print()
                for i, line in enumerate(logs, 1):
                    print(f"{i:3d}: {line}")
                print()
                input("Press Enter to continue...")
            else:
                print("Unknown command.")
                input("Press Enter to continue...")
        except KeyboardInterrupt:
            break
        except EOFError:
            break

def view_combined_logs():
    """View all logs combined with scraper prefixes."""
    while True:
        clear_screen()
        print("=" * 60)
        print("           COMBINED LOGS")
        print("=" * 60)
        print()
        
        with log_lock:
            all_logs = []
            for scraper_name, logs in scraper_logs.items():
                for line in logs:
                    all_logs.append(f"[{scraper_name}] {line}")
        
        if not all_logs:
            print("No logs available.")
        else:
            # Sort by timestamp if available, otherwise just display
            # For now, just display in order they were collected
            for i, line in enumerate(all_logs[-100:], len(all_logs)-99 if len(all_logs) > 100 else 1):  # Show last 100 lines
                print(f"{i:3d}: {line}")
            
            if len(all_logs) > 100:
                print(f"\n... ({len(all_logs) - 100} more lines)")
        
        print()
        print("Commands:")
        print("  'r' - Refresh logs")
        print("  'a' - View all combined logs")
        print("  'b' - Back to scraper list")
        print()
        
        try:
            cmd = input("Command: ").strip().lower()
            if cmd == 'b':
                break
            elif cmd == 'r':
                continue
            elif cmd == 'a':
                # Show all logs
                clear_screen()
                print("=" * 60)
                print("           ALL COMBINED LOGS")
                print("=" * 60)
                print()
                for i, line in enumerate(all_logs, 1):
                    print(f"{i:3d}: {line}")
                print()
                input("Press Enter to continue...")
            else:
                print("Unknown command.")
                input("Press Enter to continue...")
        except KeyboardInterrupt:
            break
        except EOFError:
            break

def run_all_scrapers_parallel(config):
    """Run all scrapers in parallel with real-time log viewing."""
    import threading
    import time

    print("\n" + "=" * 60)
    print("RUNNING ALL SCRAPERS IN PARALLEL")
    print("=" * 60)
    print()

    # Clear previous logs and status
    with log_lock:
        scraper_logs.clear()
        scraper_status.clear()

    # Prepare all scraper tasks
    tasks = []
    for key, scraper in SCRAPERS.items():
        scraper_config = get_scraper_config(config, key)
        
        # Get folder location
        folder_path = get_folder_location(scraper, config, scraper_config)
        if folder_path is None:
            print(f"Skipping {scraper['name']} - folder configuration failed")
            continue

        # Get additional options
        options = get_additional_options(config, scraper_config, parallel=True)
        if options is None:
            print(f"Skipping {scraper['name']} - options configuration failed")
            continue

        tasks.append((scraper, folder_path, options, config, scraper_config))

    if not tasks:
        print("No scrapers to run!")
        input("Press Enter to continue...")
        return 'menu'

    # Initialize log storage and status for all scrapers
    with log_lock:
        for scraper, _, _, _, _ in tasks:
            scraper_logs[scraper['name']] = []
            scraper_status[scraper['name']] = 'running'

    print(f"Starting {len(tasks)} scrapers in parallel...")
    print("Launching real-time log viewer...")
    time.sleep(1)  # Brief pause to show message

    # Run all scrapers in parallel
    threads = []
    results = {}

    def run_single_scraper(scraper, folder_path, options, config, scraper_config):
        try:
            result = confirm_and_run(scraper, folder_path, options, config, scraper_config, parallel=True)
            results[scraper['name']] = ('completed', result)
            with log_lock:
                scraper_status[scraper['name']] = 'completed'
        except BaseException as e:
            if isinstance(e, SystemExit):
                if e.code == 0 or e.code is None:
                    results[scraper['name']] = ('completed', None)
                    with log_lock:
                        scraper_status[scraper['name']] = 'completed'
                else:
                    results[scraper['name']] = ('error', f'exit code {e.code}')
                    with log_lock:
                        scraper_status[scraper['name']] = 'failed'
            else:
                results[scraper['name']] = ('error', str(e))
                with log_lock:
                    scraper_status[scraper['name']] = 'failed'

    # Start all threads
    for task in tasks:
        thread = threading.Thread(target=run_single_scraper, args=task)
        thread.daemon = True
        threads.append(thread)
        thread.start()

    # Show real-time log viewer while scrapers run
    try:
        curses.wrapper(log_viewer_curses_realtime, threads)
    except Exception as e:
        print(f"Real-time viewer failed: {e}")
        # Fallback to waiting
        try:
            while threads:
                threads = [t for t in threads if t.is_alive()]
                if threads:
                    print(f"Running: {len(threads)} scrapers still active...")
                    time.sleep(2)
        except KeyboardInterrupt:
            print("\n⚠ Interrupted by user - waiting for scrapers to finish...")
            for thread in threads:
                thread.join(timeout=10)
        try:
            while threads:
                threads = [t for t in threads if t.is_alive()]
                if threads:
                    print(f"Running: {len(threads)} scrapers still active...")
                    time.sleep(2)
        except KeyboardInterrupt:
            print("\n⚠ Interrupted by user - waiting for scrapers to finish...")
            for thread in threads:
                thread.join(timeout=10)

    # Print final results
    print("\n" + "=" * 60)
    print("PARALLEL SCRAPING RESULTS")
    print("=" * 60)
    
    successful = 0
    failed = 0
    
    for scraper_name, (status, details) in results.items():
        if status == 'completed':
            print(f"✓ {scraper_name}: Completed")
            successful += 1
        else:
            print(f"✗ {scraper_name}: Error - {details}")
            failed += 1

    print(f"\nSummary: {successful} successful, {failed} failed")
    
    # Pause to let user see results
    try:
        input("Press Enter to continue...")
    except KeyboardInterrupt:
        pass
    
    return 'menu'

def main():
    """Main application loop."""
    config = load_config()

    while True:
        choice = get_scraper_choice()
        if choice is None:
            print("\nGoodbye!")
            break

        if choice == 'all':
            # Run all scrapers in parallel
            result = run_all_scrapers_parallel(config)
            if result != 'menu':
                break
            continue

        scraper = SCRAPERS[choice]
        scraper_config = get_scraper_config(config, choice)
        
        print(f"\nSelected: {scraper['name']}")
        print(f"Config loaded: {bool(scraper_config)}")
        if scraper_config:
            print(f"  Folder: {scraper_config.get('folder', 'Not set')}")
            print(f"  Auto-confirm: {scraper_config.get('auto_confirm', 'Not set')}")
        else:
            print("  No scraper-specific config found")
        print(f"Global auto-confirm: {config.get('defaults', {}).get('auto_confirm', 'Not set')}")

        # Get folder location
        folder_path = get_folder_location(scraper, config, scraper_config)
        if folder_path is None:
            continue

        # Get additional options
        options = get_additional_options(config, scraper_config)
        if options is None:
            continue

        # Confirm and run scraper
        result = confirm_and_run(scraper, folder_path, options, config, scraper_config)
        if result != 'menu':
            break

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\nGoodbye!")
        sys.exit(0)
    except Exception as e:
        print(f"\nUnexpected error: {e}")
        sys.exit(1)

# Test function to populate fake logs for testing the viewer
def test_log_viewer():
    """Test the log viewer with fake data."""
    with log_lock:
        scraper_logs.clear()
        scraper_logs['Asura Scans'] = [
            '[INFO]    Starting Asura Scans scraper',
            '[INFO]    Found 15 chapters to download',
            '[SUCCESS] Downloaded chapter 1/15',
            '[SUCCESS] Downloaded chapter 2/15',
            '[WARNING] Chapter 3 failed, retrying...',
            '[SUCCESS] Downloaded chapter 3/15',
            '[INFO]    ✓ Asura Scans completed successfully!'
        ]
        scraper_logs['Flame Comics'] = [
            '[INFO]    Starting Flame Comics scraper',
            '[ERROR]   Failed to connect to website',
            '[ERROR]   Network timeout',
            '[INFO]    ✗ Flame Comics failed with exit code 1'
        ]
        scraper_logs['HiveToons'] = [
            '[INFO]    Starting HiveToons scraper',
            '[INFO]    Found 8 chapters to download',
            '[SUCCESS] Downloaded chapter 1/8',
            '[SUCCESS] Downloaded chapter 2/8',
            '[SUCCESS] Downloaded chapter 3/8',
            '[SUCCESS] Downloaded chapter 4/8',
            '[SUCCESS] Downloaded chapter 5/8',
            '[SUCCESS] Downloaded chapter 6/8',
            '[SUCCESS] Downloaded chapter 7/8',
            '[SUCCESS] Downloaded chapter 8/8',
            '[INFO]    ✓ HiveToons completed successfully!'
        ]
    
    show_log_viewer()

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1 and sys.argv[1] == 'test':
        test_log_viewer()
    else:
        try:
            main()
        except KeyboardInterrupt:
            print("\n\nGoodbye!")
            sys.exit(0)
        except Exception as e:
            print(f"\nUnexpected error: {e}")
            sys.exit(1)