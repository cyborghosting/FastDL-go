# FastDL-go

FastDL-go is a lightweight and efficient server written in Go for managing and serving game files, particularly for Source Engine-based games. It is designed to be fast, reliable, and easy to configure.

## Features
- **Game File Management**: Serve game files efficiently.
- **Configuration Support**: Easily configurable using JSON files.
- **Middleware**: Custom middleware for enhanced functionality.
- **Search Path Management**: Manage search paths for game files.

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/FastDL-go.git
   ```

2. Navigate to the project directory:
   ```bash
   cd FastDL-go
   ```

3. Build the project:
   ```bash
   go build -o fastdl ./cmd/fastdl
   ```

## Usage

1. Run the server:
   ```bash
   ./fastdl
   ```

2. Configure the server using the `configs/configuration.json` file.

## Project Structure

- `cmd/fastdl/`: Contains the main application code.
- `configs/`: Configuration files.
- `tmp/`: Temporary files and logs.

## Configuration

The server is configured using a JSON file located at `configs/configuration.json`. Update this file to set up your server preferences.

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request.

## License

This project is licensed under the MIT License. See the LICENSE file for details.

## Contact

For any inquiries, please contact [monera@gaez.uk](mailto:monera@gaez.uk).
