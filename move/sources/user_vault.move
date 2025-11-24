module datax::user_vault {
    use std::signer;
    use std::vector;

    struct Vault has key {
        datasets: vector<u64>
    }

    /// Initialize vault for a user
    public entry fun init(owner: &signer) {
        let owner_addr = signer::address_of(owner);
        if (!exists<Vault>(owner_addr)) {
            move_to(owner, Vault { datasets: vector::empty() });
        };
    }

    /// Add a dataset to user's vault
    public entry fun add_dataset(owner: &signer, dataset_id: u64) acquires Vault {
        let vault = borrow_global_mut<Vault>(signer::address_of(owner));

        // Check if dataset already exists to avoid duplicates
        let len = vector::length(&vault.datasets);
        let i = 0;
        while (i < len) {
            if (*vector::borrow(&vault.datasets, i) == dataset_id) {
                return // Already exists
            };
            i = i + 1;
        };

        vector::push_back(&mut vault.datasets, dataset_id);
    }

    /// Remove a dataset from user's vault
    public entry fun remove_dataset(owner: &signer, dataset_id: u64) acquires Vault {
        let vault = borrow_global_mut<Vault>(signer::address_of(owner));
        let datasets = &mut vault.datasets;
        let len = vector::length(datasets);

        let i = 0;
        while (i < len) {
            if (*vector::borrow(datasets, i) == dataset_id) {
                vector::remove(datasets, i);
                return
            };
            i = i + 1;
        };
    }

    /// Get all dataset IDs for a user
    public fun get_datasets(user: address): vector<u64> acquires Vault {
        if (!exists<Vault>(user)) {
            return vector::empty()
        };
        let vault = borrow_global<Vault>(user);
        vault.datasets
    }

    /// Check if user has a specific dataset
    public fun has_dataset(user: address, dataset_id: u64): bool acquires Vault {
        if (!exists<Vault>(user)) {
            return false
        };
        let vault = borrow_global<Vault>(user);
        let datasets = &vault.datasets;
        let len = vector::length(datasets);

        let i = 0;
        while (i < len) {
            if (*vector::borrow(datasets, i) == dataset_id) {
                return true
            };
            i = i + 1;
        };
        false
    }

    /// Get dataset count for a user
    public fun get_dataset_count(user: address): u64 acquires Vault {
        if (!exists<Vault>(user)) {
            return 0
        };
        let vault = borrow_global<Vault>(user);
        vector::length(&vault.datasets)
    }
}

