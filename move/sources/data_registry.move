module datax::data_registry {
    use std::signer;
    use std::vector;
    use aptos_framework::event;
    use aptos_framework::account;
    use aptos_data_network::UserVault;

    /// Event storing a hash reference to user data
    struct DataSubmitted has copy, drop, store {
        user: address,
        dataset_id: u64,
        data_hash: vector<u8>,
        metadata: vector<u8>
    }

    struct DataDeleted has copy, drop, store {
        user: address,
        dataset_id: u64
    }

    /// Dataset information stored on-chain
    struct Dataset has store {
        id: u64,
        owner: address,
        data_hash: vector<u8>,
        metadata: vector<u8>,
        created_at: u64,
        is_active: bool
    }

    struct DataStore has key {
        events: event::EventHandle<DataSubmitted>,
        delete_events: event::EventHandle<DataDeleted>,
        datasets: vector<Dataset>,
        next_dataset_id: u64
    }

    /// Initialize data store for a user (called automatically on first data submission)
    fun ensure_initialized(user: address) {
        if (!exists<DataStore>(user)) {
            // This will be called from entry function with signer
            abort 1 // Should not happen in entry function
        };
    }

    /// Initialize data store for a user
    public entry fun init(user: &signer) {
        let user_addr = signer::address_of(user);
        if (!exists<DataStore>(user_addr)) {
            move_to(
                user,
                DataStore {
                    events: account::new_event_handle<DataSubmitted>(user),
                    delete_events: account::new_event_handle<DataDeleted>(user),
                    datasets: vector::empty(),
                    next_dataset_id: 0
                }
            );
            // Also initialize user vault
            UserVault::init(user);
        };
    }

    /// Users submit hashed data reference with metadata
    public entry fun submit_data(
        user: &signer, data_hash: vector<u8>, metadata: vector<u8>
    ) acquires DataStore {
        let user_addr = signer::address_of(user);

        // Ensure initialized
        if (!exists<DataStore>(user_addr)) {
            init(user);
        };

        let store = borrow_global_mut<DataStore>(user_addr);
        let dataset_id = store.next_dataset_id;
        let timestamp = aptos_framework::timestamp::now_seconds();

        // Create dataset
        let dataset = Dataset {
            id: dataset_id,
            owner: user_addr,
            data_hash,
            metadata,
            created_at: timestamp,
            is_active: true
        };

        vector::push_back(&mut store.datasets, dataset);
        store.next_dataset_id = dataset_id + 1;

        // Read back values from stored dataset for event
        let stored_dataset = vector::borrow(&store.datasets, dataset_id);

        // Emit event
        event::emit_event(
            &mut store.events,
            DataSubmitted {
                user: user_addr,
                dataset_id,
                data_hash: stored_dataset.data_hash,
                metadata: stored_dataset.metadata
            }
        );

        // Add to user vault (will initialize if needed)
        UserVault::init(user);
        UserVault::add_dataset(user, dataset_id);
    }

    /// Get dataset information
    public fun get_dataset(
        user: address, dataset_id: u64
    ): (vector<u8>, vector<u8>, u64, bool) acquires DataStore {
        let store = borrow_global<DataStore>(user);
        let datasets = &store.datasets;
        let len = vector::length(datasets);

        let i = 0;
        while (i < len) {
            let dataset = vector::borrow(datasets, i);
            if (dataset.id == dataset_id) {
                return (
                    dataset.data_hash,
                    dataset.metadata,
                    dataset.created_at,
                    dataset.is_active
                )
            };
            i = i + 1;
        };

        abort 2 // Dataset not found
    }

    /// Delete/revoke a dataset (user has full control)
    public entry fun delete_dataset(user: &signer, dataset_id: u64) acquires DataStore {
        let user_addr = signer::address_of(user);
        let store = borrow_global_mut<DataStore>(user_addr);
        let datasets = &mut store.datasets;
        let len = vector::length(datasets);

        let i = 0;
        while (i < len) {
            let dataset = vector::borrow_mut(datasets, i);
            if (dataset.id == dataset_id
                && dataset.owner == user_addr
                && dataset.is_active) {
                dataset.is_active = false;

                // Emit deletion event
                event::emit_event(
                    &mut store.delete_events,
                    DataDeleted { user: user_addr, dataset_id }
                );

                // Remove from vault (vault should exist if dataset exists)
                UserVault::remove_dataset(user, dataset_id);
                return
            };
            i = i + 1;
        };

        abort 3 // Dataset not found or not owned by user
    }

    /// Get number of datasets for a user
    public fun get_dataset_count(user: address): u64 acquires DataStore {
        if (!exists<DataStore>(user)) {
            return 0
        };
        let store = borrow_global<DataStore>(user);
        vector::length(&store.datasets)
    }
}

