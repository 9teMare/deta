# DataX Contract Deployment

## Deployment Information

**Network:** Aptos Testnet  
**Deployed Address:** `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab`  
**Transaction Hash:** `0x027464318128f4d1462d6eae2629da4b37065b761967b2696853511bc3518ed1`  
**Explorer:** https://explorer.aptoslabs.com/txn/0x027464318128f4d1462d6eae2629da4b37065b761967b2696853511bc3518ed1?network=testnet

## Deployed Modules

1. **data_registry** - `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab::data_registry`
2. **AccessControl** - `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab::AccessControl`
3. **UserVault** - `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab::UserVault`
4. **data_token** - `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab::data_token`

## Configuration

### Frontend
All frontend components have been updated to use the deployed address:
- `app/page.tsx`
- `components/data-operations.tsx`
- `components/csv-upload.tsx`

### Backend
Backend configuration updated in `config/config.go`:
- `DataXModuleAddr`: `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab`
- `NetworkModuleAddr`: `0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab`

## Next Steps

1. Test the "Initialize User Account" button in the frontend
2. Test CSV upload functionality
3. Test data submission and vault viewing
4. Test access control features

