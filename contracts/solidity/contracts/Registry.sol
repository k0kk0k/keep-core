pragma solidity ^0.5.4;

import "openzeppelin-solidity/contracts/ownership/Ownable.sol";


/**
 * @title Registry
 * @dev An ownable contract to keep registry of approved contracts and roles.
 */
contract Registry is Ownable {
    // Registry Keeper maintains approved operator contracts. Each operator
    // contract must be approved before it can be authorized by a staker or
    // used by a service contract.
    address internal registryKeeper;

    // The Panic Button can disable malicious or malfunctioning contracts
    // that have been previously approved by the Registry Keeper.
    address internal panicButton;

    // Operator Contract Upgrader can add approved operator contracts to
    // the service contract and deprecate old ones.
    address public operatorContractUpgrader;

    // The registry of operator contracts
    // 0 - NULL (default), 1 - APPROVED, 2 - DISABLED
    mapping(address => uint256) public operatorContracts;

    modifier onlyRegistryKeeper() {
        require(registryKeeper == msg.sender, "Not authorized");
        _;
    }

    modifier onlyPanicButton() {
        require(panicButton == msg.sender, "Not authorized");
        _;
    }

    constructor() Ownable() public {
        registryKeeper = msg.sender;
        panicButton = msg.sender;
        operatorContractUpgrader = msg.sender;
    }

    function setRegistryKeeper(address _registryKeeper) public onlyOwner {
        registryKeeper = _registryKeeper;
    }

    function setPanicButton(address _panicButton) public onlyOwner {
        panicButton = _panicButton;
    }

    function setOperatorContractUpgrader(address _operatorContractUpgrader) public onlyOwner {
        operatorContractUpgrader = _operatorContractUpgrader;
    }

    function approveOperatorContract(address operatorContract) public onlyRegistryKeeper {
        operatorContracts[operatorContract] = 1;
    }

    function disableOperatorContract(address operatorContract) public onlyPanicButton {
        operatorContracts[operatorContract] = 2;
    }

    function isApprovedOperatorContract(address operatorContract) public view returns (bool) {
        if (operatorContracts[operatorContract] == 1) {
            return true;
        } else {
            return false;
        }
    }
}
