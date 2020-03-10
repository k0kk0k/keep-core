import mineBlocks from './helpers/mineBlocks';
import expectThrow from './helpers/expectThrow';

const KeepToken = artifacts.require('./KeepToken.sol');
const TokenStaking = artifacts.require('./TokenStaking.sol');
const Registry = artifacts.require("./Registry.sol");

const BN = web3.utils.BN

const chai = require('chai')
chai.use(require('bn-chai')(BN))
const expect = chai.expect

contract.only('TokenStaking', function(accounts) {

  let token, registry, stakingContract,
    account_one = accounts[0],
    account_one_operator = accounts[1],
    account_one_magpie = accounts[2],
    account_one_authorizer = accounts[3],
    account_two = accounts[4];

  const initializationPeriod = 10;
  const undelegationPeriod = 30;

  const stakingAmount = web3.utils.toBN(10000000);

  before(async () => {
    token = await KeepToken.new();
    registry = await Registry.new();
    stakingContract = await TokenStaking.new(
      token.address, registry.address, initializationPeriod, undelegationPeriod
    );
  });

  it("should send tokens correctly", async function() {
    let amount = web3.utils.toBN(1000000000);

    // Starting balances
    let account_one_starting_balance = await token.balanceOf.call(account_one);
    let account_two_starting_balance = await token.balanceOf.call(account_two);

    // Send tokens
    await token.transfer(account_two, amount, {from: account_one});

    // Ending balances
    let account_one_ending_balance = await token.balanceOf.call(account_one);
    let account_two_ending_balance = await token.balanceOf.call(account_two);

    expect(account_one_ending_balance).to.eq.BN(
      account_one_starting_balance.sub(amount), 
      "Amount wasn't correctly taken from the sender"
    )
    expect(account_two_ending_balance).to.eq.BN(
      account_two_starting_balance.add(amount), 
      "Amount wasn't correctly sent to the receiver"
    );
  });

  it("should allow to cancel delegation", async () => {
    // Starting balances
    let account_one_starting_balance = await token.balanceOf.call(account_one);

    let data = Buffer.concat([
      Buffer.from(account_one_magpie.substr(2), 'hex'),
      Buffer.from(account_one_operator.substr(2), 'hex'),
      Buffer.from(account_one_authorizer.substr(2), 'hex')
    ]);
    
    await token.approveAndCall(
      stakingContract.address, stakingAmount, 
      '0x' + data.toString('hex'), 
      {from: account_one}
    );
    
    // Ending balances
    let account_one_ending_balance = await token.balanceOf.call(account_one);
    let account_one_operator_stake_balance = await stakingContract.balanceOf.call(account_one_operator);
    
    expect(account_one_ending_balance).to.eq.BN(
      account_one_starting_balance.sub(stakingAmount),
      "Staking amount should be transferred from owner balance"
    );
    expect(account_one_operator_stake_balance).to.eq.BN(
      stakingAmount,
      "Staking amount should be added to the operator balance"
    );
    
    // Cancel stake
    await stakingContract.cancelStake(account_one_operator, {from: account_one});
    expect(account_one_starting_balance).to.eq.BN(
      await token.balanceOf.call(account_one),
      "Staking amount should be transferred back to owner"
    );
    expect(await stakingContract.balanceOf.call(account_one_operator)).to.eq.BN( 
      0, 
      "Staking amount should be removed from operator balance"
    );
  })

  it("should stake delegate and undelegate tokens correctly", async function() {
    // Starting balances
    let account_one_starting_balance = await token.balanceOf.call(account_one);

    let data = Buffer.concat([
      Buffer.from(account_one_magpie.substr(2), 'hex'),
      Buffer.from(account_one_operator.substr(2), 'hex'),
      Buffer.from(account_one_authorizer.substr(2), 'hex')
    ]);

    // Stake tokens using approveAndCall pattern
    await token.approveAndCall(stakingContract.address, stakingAmount, '0x' + data.toString('hex'), {from: account_one});

    // jump in time, full initialization period
    await mineBlocks(initializationPeriod);

    // Can not cancel stake
    await expectThrow(stakingContract.cancelStake(account_one_operator, {from: account_one}));

    // Undelegate tokens as operator
    await stakingContract.undelegate(account_one_operator, {from: account_one_operator});

    // should not be able to recover stake
    await expectThrow(stakingContract.recoverStake(account_one_operator));

    // jump in time, full undelegation period
    await mineBlocks(undelegationPeriod);

    // should be able to recover stake
    await stakingContract.recoverStake(account_one_operator);

    // should fail cause there is no stake to recover
    await expectThrow(stakingContract.recoverStake(account_one_operator));

    // check balances
    let account_one_ending_balance = await token.balanceOf.call(account_one);
    let account_one_operator_stake_balance = await stakingContract.balanceOf.call(account_one_operator);

    expect(account_one_ending_balance).to.eq.BN(
      account_one_starting_balance, 
      "Staking amount should be transfered to sender balance"
    );
    expect(account_one_operator_stake_balance).to.eq.BN(
      0, 
      "Staking amount should be removed from sender staking balance"
    );

    // Starting balances
    account_one_starting_balance = await token.balanceOf.call(account_one);

    data = Buffer.concat([
      Buffer.from(account_one_magpie.substr(2), 'hex'),
      Buffer.from(account_one_operator.substr(2), 'hex'),
      Buffer.from(account_one_authorizer.substr(2), 'hex')
    ]);

    // Stake tokens using approveAndCall pattern
    await token.approveAndCall(stakingContract.address, stakingAmount, '0x' + data.toString('hex'), {from: account_one});

    // Ending balances
    account_one_ending_balance = await token.balanceOf.call(account_one);
    account_one_operator_stake_balance = await stakingContract.balanceOf.call(account_one_operator);

    expect(account_one_ending_balance).to.eq.BN(
      account_one_starting_balance.sub(stakingAmount), 
      "Staking amount should be transfered from sender balance for the second time"
    );
    expect(account_one_operator_stake_balance).to.eq.BN(
      stakingAmount, 
      "Staking amount should be added to the sender staking balance for the second time"
    );
  });
});
