# DataX Frontend

A Next.js frontend for the DataX decentralized data network, built with Tailwind CSS and shadcn/ui.

## Features

- 🎨 Modern UI with Tailwind CSS and shadcn/ui
- 🔐 Wallet connection (Aptos wallet support)
- 📊 Data submission and management
- 🔑 Access control (grant/revoke access)
- 💼 User vault viewing
- 🎯 Real-time transaction feedback

## Getting Started

### Prerequisites

- Node.js 18+ and pnpm
- Backend server running on `http://localhost:8080`

### Installation

1. Install dependencies:
```bash
pnpm install
```

2. Create `.env.local` file:
```bash
cp .env.local.example .env.local
```

3. Update `.env.local` with your backend URL if different:
```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

4. Run the development server:
```bash
pnpm dev
```

5. Open [http://localhost:3000](http://localhost:3000) in your browser.

## Project Structure

```
frontend/
├── app/
│   ├── layout.tsx          # Root layout with providers
│   ├── page.tsx            # Main page
│   └── globals.css          # Global styles
├── components/
│   ├── ui/                  # shadcn/ui components
│   ├── wallet-connect.tsx  # Wallet connection component
│   ├── data-operations.tsx # Data operations component
│   ├── access-control.tsx  # Access control component
│   └── vault-view.tsx       # Vault viewing component
├── lib/
│   ├── api.ts              # API client
│   ├── utils.ts            # Utility functions
│   └── wallet.tsx          # Wallet provider
└── public/                 # Static assets
```

## Features

### Wallet Connection
- Connect using private key (for testing)
- View connected account address
- Disconnect wallet

### Data Operations
- Submit data with hash and metadata
- Delete datasets you own
- View transaction hashes

### Access Control
- Grant access to requesters with expiration
- Revoke access from requesters
- Check if a requester has access

### Vault Management
- View all datasets in your vault
- See dataset count
- Refresh vault data

## Wallet Integration

Currently, the frontend uses a simple private key input for testing. For production, you should:

1. Install Aptos wallet adapter packages:
```bash
pnpm install @aptos-labs/wallet-adapter-react @aptos-labs/wallet-adapter-ant-design
```

2. Integrate with Petra, Pontem, or other Aptos wallets
3. Use wallet's signMessage/signTransaction methods instead of private key

## API Integration

The frontend connects to the backend API at the URL specified in `NEXT_PUBLIC_API_URL`. All API calls are handled through the `apiClient` in `lib/api.ts`.

## Security Notes

⚠️ **Important**: The current implementation uses private keys directly in the frontend. This is **NOT SECURE** for production:

1. Never expose private keys in client-side code
2. Use proper wallet integration (Petra, Pontem, etc.)
3. Implement proper authentication
4. Use environment variables for sensitive configuration
5. Never commit private keys to version control

## Development

### Adding New Components

Use shadcn/ui CLI to add components:
```bash
pnpm dlx shadcn-ui@latest add [component-name]
```

### Styling

- Uses Tailwind CSS for styling
- shadcn/ui components for UI elements
- Dark mode support included

## License

MIT

