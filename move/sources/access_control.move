module datax::access_control {
    use std::vector;
    use std::signer;

    struct Access has store, drop {
        dataset_id: u64,
        requester: address,
        expires_at: u64
    }

    struct AccessList has key {
        entries: vector<Access>
    }

    /// Initialize access control for a user
    public entry fun init(owner: &signer) {
        let owner_addr = signer::address_of(owner);
        if (!exists<AccessList>(owner_addr)) {
            move_to(owner, AccessList { entries: vector::empty() });
        };
    }

    /// Grant access to a requester for a specific dataset with expiration
    public entry fun grant_access(
        owner: &signer,
        dataset_id: u64,
        requester: address,
        expires_at: u64
    ) acquires AccessList {
        let owner_addr = signer::address_of(owner);

        // Ensure initialized
        if (!exists<AccessList>(owner_addr)) {
            init(owner);
        };

        let list = borrow_global_mut<AccessList>(owner_addr);

        // Check if access already exists and update it, or add new
        let len = vector::length(&list.entries);
        let i = 0;
        while (i < len) {
            let entry = vector::borrow_mut(&mut list.entries, i);
            if (entry.dataset_id == dataset_id && entry.requester == requester) {
                // Update existing access
                entry.expires_at = expires_at;
                return
            };
            i = i + 1;
        };

        // Add new access
        let entry = Access { dataset_id, requester, expires_at };
        vector::push_back(&mut list.entries, entry);
    }

    /// Revoke access for a requester (user has full control)
    public entry fun revoke_access(
        owner: &signer, dataset_id: u64, requester: address
    ) acquires AccessList {
        let list = borrow_global_mut<AccessList>(signer::address_of(owner));
        let entries = &mut list.entries;
        let len = vector::length(entries);

        let i = 0;
        while (i < len) {
            let entry = vector::borrow(entries, i);
            if (entry.dataset_id == dataset_id && entry.requester == requester) {
                vector::remove(entries, i);
                return
            };
            i = i + 1;
        };
    }

    /// Check if a requester has access to a dataset (checks expiration)
    public fun has_access(
        owner: address, dataset_id: u64, requester: address
    ): bool acquires AccessList {
        if (!exists<AccessList>(owner)) {
            return false
        };

        let list = borrow_global<AccessList>(owner);
        let current_time = aptos_framework::timestamp::now_seconds();

        let i = 0;
        let len = vector::length(&list.entries);
        while (i < len) {
            let entry = vector::borrow(&list.entries, i);
            if (entry.dataset_id == dataset_id && entry.requester == requester) {
                // Check if access has expired
                if (entry.expires_at >= current_time) {
                    return true
                } else {
                    return false // Access expired
                }
            };
            i = i + 1;
        };

        false
    }

    /// Get all access entries for a dataset
    public fun get_access_list(owner: address, dataset_id: u64): vector<address> acquires AccessList {
        let result = vector::empty<address>();

        if (!exists<AccessList>(owner)) {
            return result
        };

        let list = borrow_global<AccessList>(owner);
        let current_time = aptos_framework::timestamp::now_seconds();

        let i = 0;
        let len = vector::length(&list.entries);
        while (i < len) {
            let entry = vector::borrow(&list.entries, i);
            if (entry.dataset_id == dataset_id && entry.expires_at >= current_time) {
                vector::push_back(&mut result, entry.requester);
            };
            i = i + 1;
        };

        result
    }
}

