// SPDX-License-Identifier: MIT
pragma solidity ^0.5.0;

contract TestEmitter {
    event TestEvent(uint256 indexed id, address indexed sender, string data);
    
    function emitEvent(uint256 id, string memory data) public {
        emit TestEvent(id, msg.sender, data);
    }
    
    function emitMultipleEvents(uint256 startId, uint256 count, string memory data) public {
        for (uint256 i = 0; i < count; i++) {
            emit TestEvent(startId + i, msg.sender, data);
        }
    }
}
